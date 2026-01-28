package adsb

import (
	"reflect"

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
		if previous.ADSB.Lat != current.ADSB.Lat {
			delta["lat"] = current.ADSB.Lat
		}
		if previous.ADSB.Lon != current.ADSB.Lon {
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
		if previous.ADSB.GS != current.ADSB.GS {
			delta["gs"] = current.ADSB.GS
		}

		// True Airspeed
		if previous.ADSB.TAS != current.ADSB.TAS {
			delta["tas"] = current.ADSB.TAS
		}

		// Barometric Rate
		if previous.ADSB.BaroRate != current.ADSB.BaroRate {
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

	// Compare phase data
	if !reflect.DeepEqual(previous.Phase, current.Phase) {
		delta["phase"] = current.Phase
	}

	// Compare distance
	if (previous.Distance == nil) != (current.Distance == nil) ||
		(previous.Distance != nil && current.Distance != nil && *previous.Distance != *current.Distance) {
		delta["distance"] = current.Distance
	}

	// NOTE: LastSeen is intentionally NOT compared - it changes every cycle
	// and would cause unnecessary updates

	return delta
}
