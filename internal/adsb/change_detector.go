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
	Type     string // "added", "updated", "removed"
	Aircraft *Aircraft
	Hex      string
	// Removed Changes field - we now always send full aircraft data
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
			// Check for ANY updates (no thresholds)
			if cd.hasAnyChanges(previous, current) {
				changes = append(changes, AircraftChange{
					Type:     "updated",
					Aircraft: current,
					Hex:      hex,
				})
			}
		} else {
			// New aircraft
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

// hasAnyChanges compares two aircraft and returns true if ANY field changed (no thresholds)
func (cd *ChangeDetector) hasAnyChanges(previous, current *Aircraft) bool {
	// Compare ADSB data - detect ANY change, no matter how small
	if previous.ADSB != nil && current.ADSB != nil {
		// Position: ANY change in coordinates
		if previous.ADSB.Lat != current.ADSB.Lat || previous.ADSB.Lon != current.ADSB.Lon {
			return true
		}

		// Altitude: ANY change
		if previous.ADSB.AltBaro != current.ADSB.AltBaro {
			return true
		}

		// Track: ANY change
		if previous.ADSB.Track != current.ADSB.Track {
			return true
		}

		// Ground Speed: ANY change
		if previous.ADSB.GS != current.ADSB.GS {
			return true
		}

		// True Airspeed: ANY change
		if previous.ADSB.TAS != current.ADSB.TAS {
			return true
		}

		// Barometric Rate: ANY change
		if previous.ADSB.BaroRate != current.ADSB.BaroRate {
			return true
		}

		// Magnetic Heading: ANY change
		if previous.ADSB.MagHeading != current.ADSB.MagHeading {
			return true
		}

		// True Heading: ANY change
		if previous.ADSB.TrueHeading != current.ADSB.TrueHeading {
			return true
		}
	} else if (previous.ADSB == nil) != (current.ADSB == nil) {
		// ADSB data appeared or disappeared
		return true
	}

	// Compare basic aircraft properties
	if previous.Flight != current.Flight {
		return true
	}

	if previous.Status != current.Status {
		return true
	}

	if previous.OnGround != current.OnGround {
		return true
	}

	// Compare phase data
	if !reflect.DeepEqual(previous.Phase, current.Phase) {
		return true
	}

	// Compare distance - ANY change
	if (previous.Distance == nil) != (current.Distance == nil) ||
		(previous.Distance != nil && current.Distance != nil && *previous.Distance != *current.Distance) {
		return true
	}

	// Compare last_seen - this will trigger updates on every poll cycle for real-time behavior
	if !previous.LastSeen.Equal(current.LastSeen) {
		return true
	}

	// No changes detected
	return false
}
