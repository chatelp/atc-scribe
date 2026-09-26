package adsb

import (
	"testing"
	"time"
)

// A second airport 6.4 NM west of the first, on the same parallel, with a
// north-south runway: an aircraft on final to the first airport's runway 09,
// coming from the west, is nearer to the second for most of its approach.
const otherLat, otherLon = 45.0, 4.85

func otherRunways() RunwayData {
	rd := RunwayData{Airport: "YYYY", RunwayThresholds: map[string]map[string]struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}{}}
	rd.RunwayThresholds["18-36"] = map[string]struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}{
		"18": {Latitude: otherLat + 0.015, Longitude: otherLon},
		"36": {Latitude: otherLat - 0.015, Longitude: otherLon},
	}
	return rd
}

func twoAirports() []PhaseReference {
	return []PhaseReference{
		{Airport: "YYYY", Lat: otherLat, Lon: otherLon, Runways: otherRunways()}, // principal
		{Airport: "ZZZZ", Lat: airportLat, Lon: airportLon, Runways: testRunways()},
	}
}

func newTwoAirportTracker(t *testing.T) *TrajectoryTracker {
	t.Helper()
	tt := newTestTracker(t, PhaseReference{})
	tt.ref.storeAll(twoAirports())
	tt.referenceChanged()
	return tt
}

// The approach to ZZZZ's runway 09 is flown nearer to YYYY than to ZZZZ.
// Judged against the nearest airport it would be moving away from it, and never
// an approach; on ZZZZ's runway axis it is ZZZZ's.
func TestAnAircraftOnARunwayAxisIsThatAirports(t *testing.T) {
	tt := newTwoAirportTracker(t)
	ac := fly(tt, "a00010", 4.90, 4.92, 2500, 2200, 90, 140)
	if got := tt.DeterminePhase(ac, nil, nil); got != "APP" {
		t.Errorf("phase = %s, want APP", got)
	}
	if got := tt.AirportOf("a00010").Airport; got != "ZZZZ" {
		t.Errorf("airport = %s, want ZZZZ, whose runway axis it flies", got)
	}
	if !tt.runwayTrackerFor("ZZZZ").HasData() || tt.runwayTrackerFor("YYYY").HasData() {
		t.Error("the approach should count as runway evidence for ZZZZ only")
	}
}

// Off every runway axis, the nearest airport stands in.
func TestOffTheAxesTheNearestAirportStandsIn(t *testing.T) {
	tt := newTwoAirportTracker(t)
	ac := fly(tt, "a00011", 4.80, 4.82, 3000, 3000, 45, 200) // heading north-east, off both axes
	tt.DeterminePhase(ac, nil, nil)
	if got := tt.AirportOf("a00011").Airport; got != "YYYY" {
		t.Errorf("airport = %s, want the nearest, YYYY", got)
	}
}

// Once seen on an airport's axis, the aircraft stays that airport's: a go-around
// turning off the axis nearer to the other airport is still ZZZZ's.
func TestTheAxisIsRemembered(t *testing.T) {
	tt := newTwoAirportTracker(t)
	fly(tt, "a00012", 4.90, 4.92, 2500, 2200, 90, 140)
	tt.EnsureDerived("a00012") // as each fetch cycle does
	// Then off the axis, towards YYYY, for a while.
	start := time.Now()
	for i := 0; i < 40; i++ {
		lat, lon := airportLat+0.02, 4.92-0.001*float64(i)
		tt.Ingest("a00012", TrajectorySnapshot{Timestamp: start.Add(time.Duration(i) * time.Second),
			Lat: lat, Lon: lon, AltBaro: 3000, GS: 160, TAS: 160, Track: 300, Valid: true})
	}
	tt.EnsureDerived("a00012")
	if got := tt.AirportOf("a00012").Airport; got != "ZZZZ" {
		t.Errorf("airport = %s, want ZZZZ, remembered from its axis", got)
	}
}

// Cruising over an axis says nothing of where the aircraft is going.
func TestAnAxisCountsOnlyLow(t *testing.T) {
	tt := newTwoAirportTracker(t)
	fly(tt, "a00013", 4.90, 4.92, 12000, 12000, 90, 400)
	if got := tt.AirportOf("a00013").Airport; got != "YYYY" {
		t.Errorf("airport = %s, want the nearest, YYYY: at 12 000 ft the axis is not evidence", got)
	}
}
