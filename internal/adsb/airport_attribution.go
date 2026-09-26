package adsb

import (
	"math"

	"github.com/yegors/co-atc/internal/config"
)

// Which of the followed airports an aircraft is about.
//
// A receiver near Paris sees Orly and De Gaulle, 13 NM apart, and both are
// worth watching. Each aircraft is judged against one of them: its approach,
// its climb, whether it is closing or opening its distance. The nearest airport
// is wrong often enough to matter -- measured on 23/09, Le Bourget "sees" De
// Gaulle's approaches at 4 NM, and Orly's arrivals from the north pass De
// Gaulle first (docs-fr/05-decisions.md, Q40).
//
// What decides is the runway axis: an aircraft low on an airport's extended
// runway centreline, flying along it either way, is landing there or has just
// left. That is remembered for the rest of the flight. Until it happens, the
// nearest airport stands in, reconsidered at every update.

// airportFor returns the airport an aircraft at this position is about. at may
// be nil -- a first sighting has no history -- and when given, an alignment
// found now is remembered on it and one found earlier is kept.
//
// With one airport followed it returns that airport and does nothing else: the
// behaviour is the one-airport code's, unchanged.
func (tt *TrajectoryTracker) airportFor(at *AircraftTrajectory, lat, lon, track, alt float64) *PhaseReference {
	refs := tt.ref.all()
	if len(refs) == 1 {
		return &refs[0]
	}
	if i := alignedAirport(refs, lat, lon, track, alt, *tt.phasesConfig); i >= 0 {
		if at != nil {
			at.airport = refs[i].Airport
		}
		return &refs[i]
	}
	if at != nil && at.airport != "" {
		for i := range refs {
			if refs[i].Airport == at.airport {
				return &refs[i]
			}
		}
	}
	return nearestAirport(refs, lat, lon)
}

// alignedAirport returns the index of the airport on one of whose extended
// runway centrelines the aircraft is flying, or -1. The tolerances are the
// approach detection's own: within ApproachMaxDistanceNM of a threshold and
// ApproachCenterlineToleranceNM of the axis, heading within
// ApproachHeadingToleranceDeg of it -- either way, so a climb-out counts as
// much as a final -- and below ApproachMaxAltitudeFt, where no aircraft is only
// passing over. When two airports qualify, the nearer threshold wins.
func alignedAirport(refs []PhaseReference, lat, lon, track, alt float64, cfg config.FlightPhasesConfig) int {
	if track == 0 || alt <= 0 || alt > float64(cfg.ApproachMaxAltitudeFt) {
		return -1 // no heading, or no altitude: nothing to align
	}
	best, bestDist := -1, math.Inf(1)
	for i := range refs {
		if d, ok := distanceOnAxis(refs[i].Runways, lat, lon, track, cfg); ok && d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

// distanceOnAxis returns the distance to the nearest threshold of a runway
// whose extended centreline the aircraft is flying along.
func distanceOnAxis(runways RunwayData, lat, lon, track float64, cfg config.FlightPhasesConfig) (float64, bool) {
	best, found := math.Inf(1), false
	for pair, thresholds := range runways.RunwayThresholds {
		for id, t := range thresholds {
			opposite, ok := thresholds[getOppositeThreshold(id, pair)]
			if !ok {
				continue
			}
			dist := MetersToNM(Haversine(lat, lon, t.Latitude, t.Longitude))
			if dist > float64(cfg.ApproachMaxDistanceNM) || dist >= best {
				continue
			}
			heading := CalculateBearing(t.Latitude, t.Longitude, opposite.Latitude, opposite.Longitude)
			diff := math.Abs(math.Mod(track-heading+540, 360) - 180) // 0..180
			if diff > float64(cfg.ApproachHeadingToleranceDeg) && 180-diff > float64(cfg.ApproachHeadingToleranceDeg) {
				continue
			}
			off := CalculateRunwayCenterlineDistance(lat, lon,
				RunwayThreshold{ID: id, Latitude: t.Latitude, Longitude: t.Longitude}, heading)
			if off > cfg.ApproachCenterlineToleranceNM {
				continue
			}
			best, found = dist, true
		}
	}
	return best, found
}

// nearestAirport returns the followed airport nearest to a position.
func nearestAirport(refs []PhaseReference, lat, lon float64) *PhaseReference {
	best, bestDist := 0, math.Inf(1)
	for i := range refs {
		if d := Haversine(lat, lon, refs[i].Lat, refs[i].Lon); d < bestDist {
			best, bestDist = i, d
		}
	}
	return &refs[best]
}
