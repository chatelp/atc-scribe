package adsb

import (
	"github.com/yegors/co-atc/pkg/logger"
)

// ChangeDetector tracks aircraft changes between polling cycles
type ChangeDetector struct {
	previousAircraft map[string]*Aircraft
	logger           *logger.Logger
}

// NewChangeDetector creates a new change detector
func NewChangeDetector(logger *logger.Logger) *ChangeDetector {
	return &ChangeDetector{
		previousAircraft: make(map[string]*Aircraft),
		logger:           logger.Named("change-detector"),
	}
}

// AircraftChange represents a change in aircraft data
type AircraftChange struct {
	Type     string                 // "added", "updated", "removed"
	Aircraft *Aircraft              // Full object for "added", nil for others
	Hex      string                 // Aircraft hex code
	Delta    map[string]interface{} // Changed fields only (for "updated")
}

// DetectChanges compares current aircraft data with previous and returns changes
func (cd *ChangeDetector) DetectChanges(currentAircraft []*Aircraft) []AircraftChange {
	changes := []AircraftChange{}
	currentMap := make(map[string]*Aircraft)

	// Build current aircraft map
	for _, aircraft := range currentAircraft {
		currentMap[aircraft.Hex] = aircraft
	}

	// Detect new and updated aircraft
	for hex, current := range currentMap {
		if previous, exists := cd.previousAircraft[hex]; exists {
			// Compute delta - only returns non-empty if there are actual changes
			delta := cd.computeDelta(previous, current)
			if len(delta) > 0 {
				changes = append(changes, AircraftChange{
					Type:  "updated",
					Hex:   hex,
					Delta: delta,
				})
			}
		} else {
			// New aircraft - send full object
			changes = append(changes, AircraftChange{
				Type:     "added",
				Aircraft: current,
				Hex:      hex,
			})
		}
	}

	// Detect removed aircraft
	for hex := range cd.previousAircraft {
		if _, exists := currentMap[hex]; !exists {
			changes = append(changes, AircraftChange{
				Type: "removed",
				Hex:  hex,
			})
		}
	}

	// Update previous state
	cd.previousAircraft = currentMap
	return changes
}

// computeDelta returns only the fields that changed between previous and current aircraft
func (cd *ChangeDetector) computeDelta(previous, current *Aircraft) map[string]interface{} {
	delta := make(map[string]interface{})

	// Compare ADSB data
	if previous.ADSB != nil && current.ADSB != nil {
		// Position
		if !floatPtrEqual(previous.ADSB.Lat, current.ADSB.Lat) {
			delta["lat"] = current.ADSB.Lat
		}
		if !floatPtrEqual(previous.ADSB.Lon, current.ADSB.Lon) {
			delta["lon"] = current.ADSB.Lon
		}

		// Altitude
		if previous.ADSB.AltBaro != current.ADSB.AltBaro {
			delta["alt_baro"] = current.ADSB.AltBaro
		}

		// Track
		if previous.ADSB.Track != current.ADSB.Track {
			delta["track"] = current.ADSB.Track
		}

		// Ground Speed
		if !floatPtrEqual(previous.ADSB.GS, current.ADSB.GS) {
			delta["gs"] = current.ADSB.GS
		}

		// True Airspeed
		if !floatPtrEqual(previous.ADSB.TAS, current.ADSB.TAS) {
			delta["tas"] = current.ADSB.TAS
		}

		// Barometric Rate
		if !floatPtrEqual(previous.ADSB.BaroRate, current.ADSB.BaroRate) {
			delta["baro_rate"] = current.ADSB.BaroRate
		}

		// Magnetic Heading
		if previous.ADSB.MagHeading != current.ADSB.MagHeading {
			delta["mag_heading"] = current.ADSB.MagHeading
		}

		// True Heading
		if previous.ADSB.TrueHeading != current.ADSB.TrueHeading {
			delta["true_heading"] = current.ADSB.TrueHeading
		}

		// ATC derived metrics
		if !atcDerivedEqual(previous.ADSB.ATCDerived, current.ADSB.ATCDerived) {
			delta["atc_derived"] = current.ADSB.ATCDerived
		}
	} else if (previous.ADSB == nil) != (current.ADSB == nil) {
		// ADSB data appeared or disappeared - send full ADSB object
		if current.ADSB != nil {
			delta["adsb"] = current.ADSB
		} else {
			delta["adsb"] = nil
		}
	}

	// Compare basic aircraft properties
	if previous.Flight != current.Flight {
		delta["flight"] = current.Flight
	}

	if previous.Status != current.Status {
		delta["status"] = current.Status
	}

	if previous.OnGround != current.OnGround {
		delta["on_ground"] = current.OnGround
	}

	// Compare phase data (optimized - no reflection)
	if !phaseDataEqual(previous.Phase, current.Phase) {
		delta["phase"] = current.Phase
	}

	// Compare distance
	if (previous.Distance == nil) != (current.Distance == nil) ||
		(previous.Distance != nil && current.Distance != nil && *previous.Distance != *current.Distance) {
		delta["distance"] = current.Distance
	}

	// Compare BSDB data (BaseStation.sqb enrichment)
	if !bsdbDataEqual(previous.BSDB, current.BSDB) {
		delta["bsdb"] = current.BSDB
	}

	// last_seen is set client-side to current time when receiving any update

	return delta
}

// phaseDataEqual compares two PhaseData structs without using reflection
// Only compares the current phase since that's what matters for change detection
func phaseDataEqual(a, b *PhaseData) bool {
	// Both nil = equal
	if a == nil && b == nil {
		return true
	}
	// One nil, one not = not equal
	if a == nil || b == nil {
		return false
	}
	// Compare current phase arrays length
	if len(a.Current) != len(b.Current) {
		return false
	}
	// If both have current phase, compare the phase string and timestamp
	if len(a.Current) > 0 && len(b.Current) > 0 {
		if a.Current[0].Phase != b.Current[0].Phase {
			return false
		}
		if !a.Current[0].Timestamp.Equal(b.Current[0].Timestamp) {
			return false
		}
	}
	return true
}

// bsdbDataEqual compares two BSDBData structs without using reflection
// Returns true if both are equal (including both nil)
func bsdbDataEqual(a, b *BSDBData) bool {
	// Both nil = equal
	if a == nil && b == nil {
		return true
	}
	// One nil, one not = not equal
	if a == nil || b == nil {
		return false
	}
	// Compare all fields
	return a.Registration == b.Registration &&
		a.ICAOTypeCode == b.ICAOTypeCode &&
		a.OperatorFlagCode == b.OperatorFlagCode &&
		a.Manufacturer == b.Manufacturer &&
		a.Type == b.Type &&
		a.RegisteredOwners == b.RegisteredOwners
}
