package adsb

import (
	"testing"
	"time"
)

func cdgRefs() []PhaseReference {
	return []PhaseReference{{Airport: "LFPG", Runways: cdgRunways()}}
}

// onFinal is an observation d NM before a threshold of CDG, offset NM to the
// right of the extended centreline, on the given track.
func onFinal(t *testing.T, pair, end, opposite string, d, offset, track, alt float64, at time.Time) RunwayObservation {
	t.Helper()
	rw := cdgRunways()
	th, op := rw.RunwayThresholds[pair][end], rw.RunwayThresholds[pair][opposite]
	heading := CalculateBearing(th.Latitude, th.Longitude, op.Latitude, op.Longitude)
	lat, lon := pointOnAxis(th.Latitude, th.Longitude, heading+180, d)
	lat, lon = pointOnAxis(lat, lon, heading+90, offset)
	climb := -700.0
	return RunwayObservation{Lat: lat, Lon: lon, TrackDeg: track, AltFt: alt, BaroRateFPM: &climb, MaxApproachFt: 5000, At: at}
}

// pastEnd is an observation climbing out d NM past the far end of a runway
// taken off from `end`.
func pastEnd(t *testing.T, pair, end, opposite string, d, alt float64, at time.Time) RunwayObservation {
	t.Helper()
	rw := cdgRunways()
	th, op := rw.RunwayThresholds[pair][end], rw.RunwayThresholds[pair][opposite]
	heading := CalculateBearing(th.Latitude, th.Longitude, op.Latitude, op.Longitude)
	lat, lon := pointOnAxis(op.Latitude, op.Longitude, heading, d)
	climb := 2200.0
	return RunwayObservation{Lat: lat, Lon: lon, TrackDeg: heading, AltFt: alt, BaroRateFPM: &climb, MaxApproachFt: 5000, At: at}
}

func heading(t *testing.T, pair, end, opposite string) float64 {
	rw := cdgRunways()
	th, op := rw.RunwayThresholds[pair][end], rw.RunwayThresholds[pair][opposite]
	return CalculateBearing(th.Latitude, th.Longitude, op.Latitude, op.Longitude)
}

func TestAnArrivalEstablishedOn09LIsGiven09L(t *testing.T) {
	j := NewRunwayJudge()
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	h := heading(t, "09L-27R", "09L", "27R")
	// 6 NM out, 30 m off the 09L axis: the 09R threshold is nearer, the 09L axis is.
	j.Observe("abc123", cdgRefs(), onFinal(t, "09L-27R", "09L", "27R", 6, 0.016, h, 2200, t0))
	got := j.Get("ABC123")
	if got == nil || got.Arrival == nil || got.Arrival.Airport != "LFPG" || got.Arrival.Runway != "09L" {
		t.Fatalf("got %+v, want an arrival on LFPG 09L", got)
	}
}

func TestAnInterceptAcross09RIsNotEstablishedThere(t *testing.T) {
	j := NewRunwayJudge()
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	h := heading(t, "09R-27L", "09R", "27L")
	// On 09R's axis, but crossing it at 40 degrees towards 09L: not established.
	j.Observe("abc123", cdgRefs(), onFinal(t, "09R-27L", "09R", "27L", 7, 0, h-40, 2500, t0))
	if got := j.Get("abc123"); got != nil {
		t.Fatalf("crossing 09R: got %+v, want nothing yet", got)
	}
	h9l := heading(t, "09L-27R", "09L", "27R")
	j.Observe("abc123", cdgRefs(), onFinal(t, "09L-27R", "09L", "27R", 6, 0, h9l, 2200, t0.Add(time.Minute)))
	if got := j.Get("abc123"); got == nil || got.Arrival.Runway != "09L" {
		t.Fatalf("then on 09L: got %+v, want 09L", got)
	}
}

func TestALateChangeOfRunwayIsFollowed(t *testing.T) {
	j := NewRunwayJudge()
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	// FPO4UD, 29/09: on 09L at 8 NM, landed on 09R.
	j.Observe("fpo4ud", cdgRefs(), onFinal(t, "09L-27R", "09L", "27R", 7.5, 0, heading(t, "09L-27R", "09L", "27R"), 2600, t0))
	j.Observe("fpo4ud", cdgRefs(), onFinal(t, "09R-27L", "09R", "27L", 3.5, 0, heading(t, "09R-27L", "09R", "27L"), 1300, t0.Add(2*time.Minute)))
	if got := j.Get("fpo4ud"); got == nil || got.Arrival.Runway != "09R" {
		t.Fatalf("got %+v, want 09R", got)
	}
}

func TestNotOnFinalNoRunway(t *testing.T) {
	j := NewRunwayJudge()
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	h := heading(t, "09L-27R", "09L", "27R")
	for name, o := range map[string]RunwayObservation{
		"above the approach ceiling": onFinal(t, "09L-27R", "09L", "27R", 6, 0, h, 7000, t0),
		"too far out":                onFinal(t, "09L-27R", "09L", "27R", 9, 0, h, 3000, t0),
		// North of 09L: 300 m south would be 83 m from 09R's axis, and rightly 09R.
		"300 m off the axis":   onFinal(t, "09L-27R", "09L", "27R", 6, -0.16, h, 2200, t0),
		"flying the other way": onFinal(t, "09L-27R", "09L", "27R", 6, 0, h+180, 2200, t0),
	} {
		j.Observe("x"+name, cdgRefs(), o)
		if got := j.Get("x" + name); got != nil {
			t.Errorf("%s: got %+v, want nothing", name, got)
		}
	}
}

func TestADepartureIsJudgedOnItsInitialClimb(t *testing.T) {
	j := NewRunwayJudge()
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	j.Observe("dep1", cdgRefs(), pastEnd(t, "09R-27L", "09R", "27L", 2, 1500, t0))
	got := j.Get("dep1")
	if got == nil || got.Departure == nil || got.Departure.Runway != "09R" || got.Arrival != nil {
		t.Fatalf("got %+v, want a departure from 09R", got)
	}
	// Kept as it turns and climbs away.
	j.Observe("dep1", cdgRefs(), pastEnd(t, "08L-26R", "08L", "26R", 3, 2500, t0.Add(time.Minute)))
	if got := j.Get("dep1"); got.Departure.Runway != "09R" {
		t.Fatalf("got %+v, want 09R kept", got)
	}
}

func TestAGoAroundIsNotADeparture(t *testing.T) {
	j := NewRunwayJudge()
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	j.Observe("ga1", cdgRefs(), onFinal(t, "09L-27R", "09L", "27R", 3, 0, heading(t, "09L-27R", "09L", "27R"), 1200, t0))
	j.Observe("ga1", cdgRefs(), pastEnd(t, "09L-27R", "09L", "27R", 1.5, 1500, t0.Add(3*time.Minute)))
	got := j.Get("ga1")
	if got == nil || got.Arrival == nil || got.Departure != nil {
		t.Fatalf("got %+v, want the arrival only", got)
	}
}

func TestATurnaroundForgetsTheArrival(t *testing.T) {
	j := NewRunwayJudge()
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	j.Observe("rot1", cdgRefs(), onFinal(t, "08R-26L", "08R", "26L", 4, 0, heading(t, "08R-26L", "08R", "26L"), 1500, t0))
	j.Observe("rot1", cdgRefs(), pastEnd(t, "08L-26R", "08L", "26R", 2, 1800, t0.Add(55*time.Minute)))
	got := j.Get("rot1")
	if got == nil || got.Arrival != nil || got.Departure == nil || got.Departure.Runway != "08L" {
		t.Fatalf("got %+v, want only the new departure from 08L", got)
	}
}

func TestTheRunwayInUseCountsAircraftNotReports(t *testing.T) {
	rt := newChoiceTracker(t, cdgRunways())
	// One aircraft on a long final, a hundred reports; two others seen once.
	for i := 0; i < 100; i++ {
		rt.RecordEvent("09L-27R/09L", RunwayEventApproach, "long01")
	}
	rt.RecordEvent("08R-26L/08R", RunwayEventApproach, "short1")
	rt.RecordEvent("08R-26L/08R", RunwayEventApproach, "short2")
	scores := rt.GetTopScores(2)
	if len(scores) != 2 || scores[0].RunwayEnd != "08R-26L/08R" || scores[0].EventCount != 2 || scores[1].EventCount != 1 {
		t.Fatalf("got %+v, want 08R first with 2 aircraft, then 09L with 1", scores)
	}
}
