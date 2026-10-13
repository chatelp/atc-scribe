package adsb

import (
	"math"
	"testing"
	"time"
)

// A receiver that decodes a position every ~1.7 s, polled every second, hands
// the same fix to about half the polls. The live prediction must extrapolate
// from when the fix was decoded (seen_pos), not from the poll that repeated
// it: anchored on the poll, it stayed on the old fix and the page oscillated
// between it and its own extrapolation (measured 13/10/2026, 30 % of the
// rendered steps backwards).
func TestLivePredictionExtrapolatesFromPositionTime(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
	const hex = "abc123"
	const gs = 400.0 // kt, due east
	mps := gs * 0.514444
	lat := 48.5
	dLonPerSec := mps / (111320 * math.Cos(lat*math.Pi/180))
	base := time.Now().UTC().Add(-20 * time.Second)

	// Ten polls, each with a fresh fix.
	var fix TrajectorySnapshot
	for i := 0; i < 10; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		fix = TrajectorySnapshot{
			Timestamp: ts, PosTimestamp: ts,
			Lat: lat, Lon: 2.0 + float64(i)*dLonPerSec,
			AltBaro: 30000, GS: gs, TAS: gs, Track: 90, Valid: true,
		}
		tt.Ingest(hex, fix)
	}
	// Two polls that repeat the last fix, decoded at base+9 s.
	for i := 10; i < 12; i++ {
		stale := fix
		stale.Timestamp = base.Add(time.Duration(i) * time.Second)
		tt.Ingest(hex, stale)
	}
	tt.EnsureDerived(hex)

	at := func(sec float64) LivePrediction {
		t.Helper()
		for _, p := range tt.GetLivePredictionsAt(base.Add(time.Duration(sec * float64(time.Second)))) {
			if p.Hex == hex {
				return p
			}
		}
		t.Fatalf("no live prediction at +%.1f s", sec)
		return LivePrediction{}
	}
	metresEast := func(p LivePrediction) float64 {
		return (p.Lon - fix.Lon) * 111320 * math.Cos(lat*math.Pi/180)
	}

	p1 := at(10.5) // 1.5 s after the fix was decoded, 0.5 s after the second repeat
	p2 := at(11.5)
	if got, want := metresEast(p1), mps*1.5; math.Abs(got-want) > 40 {
		t.Errorf("1.5 s after the fix the prediction is %.0f m east of it, want about %.0f m", got, want)
	}
	if adv := metresEast(p2) - metresEast(p1); adv < mps*0.8 || adv > mps*1.2 {
		t.Errorf("the prediction advanced %.0f m in one second, want about %.0f m", adv, mps)
	}
}

// The snapshot dates its position from seen_pos when the source gives it.
func TestSnapshotPositionTimeFromSeenPos(t *testing.T) {
	now := time.Now().UTC()
	lat, lon, seenPos := 48.5, 2.0, 2.4
	snap := TrajectorySnapshotFromADSB(&ADSBTarget{Lat: &lat, Lon: &lon, SeenPos: &seenPos}, false, now, time.Time{})
	if got, want := snap.PositionTime(), now.Add(-2400*time.Millisecond); !got.Equal(want) {
		t.Errorf("position time %v, want %v", got, want)
	}
	plain := TrajectorySnapshotFromADSB(&ADSBTarget{Lat: &lat, Lon: &lon}, false, now, time.Time{})
	if !plain.PositionTime().Equal(now) {
		t.Errorf("without seen_pos the position time is the poll's: got %v", plain.PositionTime())
	}
}

// One model for every predicted position (fiche L, step 2): an aircraft the
// tracker cannot forecast gets the same kinematic forecast from its one
// sample, along its track at its ground speed, from when its position was
// decoded.
func TestForecastFromOneSample(t *testing.T) {
	lat, lon, gs, track, tas, heading, vr := 48.5, 2.0, 400.0, 90.0, 430.0, 95.0, -1200.0
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	target := &ADSBTarget{Lat: &lat, Lon: &lon, AltBaro: 30000, GS: &gs, Track: &track, TAS: &tas, TrueHeading: &heading, BaroRate: &vr}
	points := ForecastFromTarget(target, at)
	if len(points) != forecastSteps {
		t.Fatalf("%d points, want %d", len(points), forecastSteps)
	}
	p := points[len(points)-1]
	sec := forecastSteps * forecastStepSec
	if !p.Timestamp.Equal(at.Add(time.Duration(sec) * time.Second)) {
		t.Errorf("last point at %v, want %v s after the fix", p.Timestamp, sec)
	}
	east := (p.Lon - lon) * 111000 * math.Cos(lat*math.Pi/180)
	if want := gs * 0.514444 * sec; math.Abs(east-want) > want*0.02 || math.Abs(p.Lat-lat) > 1e-6 {
		t.Errorf("after %v s: %.0f m east, lat %+.6f; want %.0f m east along the track", sec, east, p.Lat-lat, want)
	}
	if want := 30000 + vr*sec/60; math.Abs(p.Altitude-want) > 1 {
		t.Errorf("altitude %.0f, want %.0f", p.Altitude, want)
	}
	if p.Speed != gs || p.Heading != track {
		t.Errorf("speed %v heading %v, want the ground speed %v and the track %v", p.Speed, p.Heading, gs, track)
	}

	// Nothing to predict from: no position, on the ground, or not moving.
	for name, tg := range map[string]*ADSBTarget{
		"no position": {AltBaro: 30000, GS: &gs, Track: &track},
		"no altitude": {Lat: &lat, Lon: &lon, GS: &gs, Track: &track},
		"no speed":    {Lat: &lat, Lon: &lon, AltBaro: 30000, Track: &track},
	} {
		if got := ForecastFromTarget(tg, at); got != nil {
			t.Errorf("%s: %d points, want none", name, len(got))
		}
	}
}

// The forecast the tracker computes and the one from a single sample have
// the same form once turned into positions for the API.
func TestForecastPositionsForm(t *testing.T) {
	lat, lon, gs, track := 48.5, 2.0, 400.0, 90.0
	positions := PredictionPointsToPositions(ForecastFromTarget(&ADSBTarget{Lat: &lat, Lon: &lon, AltBaro: 30000, GS: &gs, Track: &track}, time.Now()))
	if len(positions) == 0 {
		t.Fatal("no positions")
	}
	for i, p := range positions {
		if p.Lat == nil || p.Lon == nil || p.Altitude == nil || p.SpeedGS == nil || p.SpeedTrue == nil ||
			p.TrueHeading == nil || p.MagHeading == nil || p.Timestamp.IsZero() {
			t.Errorf("position %d incomplete: %+v", i, p)
		}
	}
}

// The predicted state's track is a track: between the fix and the first
// forecast point it does not start from the true heading, which carries the
// wind's drift.
func TestLivePredictionTrackIsNotTheHeading(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
	const hex = "abc124"
	base := time.Now().UTC().Add(-20 * time.Second)
	for i := 0; i < 10; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		tt.Ingest(hex, TrajectorySnapshot{
			Timestamp: ts, PosTimestamp: ts, Lat: 48.5, Lon: 2.0 + float64(i)*0.0028,
			AltBaro: 30000, GS: 400, TAS: 400, Track: 90, TrueHeading: 101, MagHeading: 99, Valid: true,
		})
	}
	tt.EnsureDerived(hex)
	for _, p := range tt.GetLivePredictionsAt(base.Add(10 * time.Second)) {
		if p.Hex == hex && math.Abs(p.Heading-90) > 0.5 {
			t.Errorf("predicted track %.1f, want the track 90, not the heading 101", p.Heading)
		}
	}
}

// turningTrack feeds twenty seconds of a constant turn, one fix a second,
// and returns the last fix.
func turningTrack(tt *TrajectoryTracker, hex string, base time.Time, rateDps float64) TrajectorySnapshot {
	const gs = 250.0
	lat, lon, track := 48.5, 2.0, 0.0
	var last TrajectorySnapshot
	for i := 0; i < 20; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		last = TrajectorySnapshot{Timestamp: ts, PosTimestamp: ts, Lat: lat, Lon: lon,
			AltBaro: 8000, GS: gs, TAS: gs, Track: track, Valid: true}
		tt.Ingest(hex, last)
		v := gs * 0.514444
		rad := (track + rateDps/2) * math.Pi / 180
		lat += v * math.Cos(rad) / 111320
		lon += v * math.Sin(rad) / (111320 * math.Cos(lat*math.Pi/180))
		track = math.Mod(track+rateDps+360, 360)
	}
	tt.EnsureDerived(hex)
	return last
}

// In a turn the forecast starts on the fix's track, not on the mean track of
// the last ten seconds, which lags half the window behind.
func TestForecastStartsOnTheFixTrack(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
	last := turningTrack(tt, "abc125", time.Now().UTC().Add(-30*time.Second), 3)
	f := tt.GetForecast("abc125")
	if len(f) == 0 {
		t.Fatal("no forecast")
	}
	// Five seconds on at 3 deg/s: 15 deg past the fix's track.
	if got, want := f[0].Heading, math.Mod(last.Track+15, 360); math.Abs(circularDiff(got, want)) > 2 {
		t.Errorf("first forecast point on track %.1f, want %.1f (the fix's %.1f, turned 5 s)", got, want, last.Track)
	}
}

// In a turn the live prediction follows the arc, not the chord to the first
// forecast point five seconds on.
func TestLivePredictionFollowsTheTurn(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
	const rate, gs = 3.0, 250.0
	base := time.Now().UTC().Add(-30 * time.Second)
	last := turningTrack(tt, "abc126", base, rate)
	// The true path on from the last fix, 2.5 s later.
	lat, lon, track := last.Lat, last.Lon, last.Track
	for i := 0; i < 250; i++ {
		v := gs * 0.514444 * 0.01
		rad := track * math.Pi / 180
		lat += v * math.Cos(rad) / 111320
		lon += v * math.Sin(rad) / (111320 * math.Cos(lat*math.Pi/180))
		track += rate * 0.01
	}
	at := last.PositionTime().Add(2500 * time.Millisecond)
	for _, p := range tt.GetLivePredictionsAt(at) {
		if p.Hex != "abc126" {
			continue
		}
		off := Haversine(p.Lat, p.Lon, lat, lon)
		if off > 6 {
			t.Errorf("2.5 s into the turn the prediction is %.1f m off the arc, want under 6 m", off)
		}
		return
	}
	t.Fatal("no live prediction in the turn")
}

// seen_pos counts from when the receiver wrote aircraft.json, not from when
// the server read it: a tar1090 or readsb file read half a second after it
// was written dates the position half a second earlier; another source, or a
// receiver clock ahead of the server's, keeps the server's time.
func TestReceiverTimeDatesTheFile(t *testing.T) {
	read := time.Date(2026, 10, 13, 12, 0, 10, 500_000_000, time.UTC)
	written := float64(read.Add(-500*time.Millisecond).UnixNano()) / 1e9
	raw := &RawAircraftData{Now: written}
	if got := ReceiverTime(raw, SourceTypeReadsbFile, read); got.Sub(read) != -500*time.Millisecond {
		t.Errorf("readsb file: %v, want 0.5 s before the read", got.Sub(read))
	}
	if got := ReceiverTime(raw, SourceTypeExternalAPI, read); !got.Equal(read) {
		t.Errorf("external source: %v, want the server's time", got)
	}
	ahead := &RawAircraftData{Now: float64(read.Add(2 * time.Second).Unix())}
	if got := ReceiverTime(ahead, SourceTypeTar1090, read); !got.Equal(read) {
		t.Errorf("receiver clock ahead: %v, want the server's time", got)
	}
	if got := ReceiverTime(&RawAircraftData{}, SourceTypeTar1090, read); !got.Equal(read) {
		t.Errorf("no now: %v, want the server's time", got)
	}

	lat, lon, seenPos := 48.5, 2.0, 1.2
	snap := TrajectorySnapshotFromADSB(&ADSBTarget{Lat: &lat, Lon: &lon, SeenPos: &seenPos}, false, read, ReceiverTime(raw, SourceTypeTar1090, read))
	if got, want := snap.PositionTime(), read.Add(-1700*time.Millisecond); !got.Equal(want) {
		t.Errorf("position time %v, want %v (file time less seen_pos)", got, want)
	}
	if !snap.Timestamp.Equal(read) {
		t.Errorf("the snapshot is still ingested at the read: %v", snap.Timestamp)
	}
}
