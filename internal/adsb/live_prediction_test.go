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
	snap := TrajectorySnapshotFromADSB(&ADSBTarget{Lat: &lat, Lon: &lon, SeenPos: &seenPos}, false, now)
	if got, want := snap.PositionTime(), now.Add(-2400*time.Millisecond); !got.Equal(want) {
		t.Errorf("position time %v, want %v", got, want)
	}
	plain := TrajectorySnapshotFromADSB(&ADSBTarget{Lat: &lat, Lon: &lon}, false, now)
	if !plain.PositionTime().Equal(now) {
		t.Errorf("without seen_pos the position time is the poll's: got %v", plain.PositionTime())
	}
}
