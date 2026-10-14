package simtraffic

import (
	"math"
	"testing"
)

func TestFleetSizes(t *testing.T) {
	for _, n := range []int{5, 12, 60, 150} {
		tr := New(Options{Aircraft: n, Seconds: 60, Seed: 1, StationLat: 45, StationLon: 4.67, StoreEvery: 10})
		if len(tr.Fleet) != n {
			t.Errorf("%d aircraft asked, %d flown", n, len(tr.Fleet))
		}
		seen := map[string]bool{}
		for _, a := range tr.Fleet {
			if seen[a.Hex] {
				t.Errorf("hex %s twice", a.Hex)
			}
			seen[a.Hex] = true
		}
	}
}

// The first twelve are the replay bench's, whatever the size of the fleet.
func TestFirstTwelveUnchanged(t *testing.T) {
	small := New(Options{Aircraft: 12, Seconds: 120, Seed: 7, StationLat: 45, StationLon: 4.67})
	large := New(Options{Aircraft: 60, Seconds: 120, Seed: 7, StationLat: 45, StationLon: 4.67})
	for i := 0; i < 12; i++ {
		for _, at := range []float64{3, 47.5, 110} {
			a, b := small.Fleet[i].Target(at), large.Fleet[i].Target(at)
			if a["lat"] != b["lat"] || a["seen_pos"] != b["seen_pos"] || a["gs"] != b["gs"] {
				t.Fatalf("%s at %v s: %v, %v", small.Fleet[i].Hex, at, a, b)
			}
		}
	}
}

// Keeping one state in ten moves the truth by centimetres.
func TestStoreEvery(t *testing.T) {
	full := New(Options{Aircraft: 30, Seconds: 90, Seed: 3, StationLat: 45, StationLon: 4.67})
	light := New(Options{Aircraft: 30, Seconds: 90, Seed: 3, StationLat: 45, StationLon: 4.67, StoreEvery: 10})
	worst := 0.0
	for i := range full.Fleet {
		for at := 0.0; at < 90; at += 0.37 {
			p, q := full.Fleet[i].TruthAt(at), light.Fleet[i].TruthAt(at)
			d := math.Hypot((p.Lat-q.Lat)*111320, (p.Lon-q.Lon)*111320*math.Cos(p.Lat*math.Pi/180))
			worst = math.Max(worst, d)
		}
	}
	if worst > 0.5 {
		t.Errorf("light truth up to %.2f m off", worst)
	}
}

// The long gap: no position decoded for 18 s, the old one repeated with
// seen_pos growing, then a fix again; and the receiver's rate.
func TestLongGapAndRate(t *testing.T) {
	tr := New(Options{Aircraft: 13, Seconds: 200, Seed: 11, StationLat: 45, StationLon: 4.67, LongGapFrom: 80})
	a := tr.Fleet[12]
	if a.Kind != "longgap" || a.GapFrom != 80 || a.GapTo != 98 {
		t.Fatalf("aircraft 13: %s %v-%v", a.Kind, a.GapFrom, a.GapTo)
	}
	d := math.Hypot((a.Lat-45)*111320, (a.Lon-4.67)*111320*math.Cos(45*math.Pi/180)) / 1852
	if math.Abs(d-45) > 0.5 {
		t.Errorf("long gap aircraft at %.1f NM", d)
	}
	in := a.Target(97)
	if in["seen_pos"].(float64) < 16 {
		t.Errorf("seen_pos %v at the end of the gap", in["seen_pos"])
	}
	after := a.Target(105)
	if after["seen_pos"].(float64) > 6 {
		t.Errorf("no fix 7 s after the gap: seen_pos %v", after["seen_pos"])
	}
	fixes := 0
	last := -1.0
	for k := 0; k < 200; k++ {
		tg := tr.Fleet[0].Target(float64(k))
		if tg["lat"] != last {
			fixes++
			last = tg["lat"].(float64)
		}
	}
	if fixes < 80 || fixes > 140 {
		t.Errorf("%d new positions in 200 polls", fixes)
	}
}
