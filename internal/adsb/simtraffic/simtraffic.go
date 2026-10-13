// Package simtraffic flies synthetic aircraft and writes them the way a
// readsb receiver answers in aircraft.json.
//
// It is the generator of the replay bench (docs-fr/fiches-cloud/L-bloc-position.md,
// internal/adsb/replay_test.go) taken out of its test file, so the browser
// bench (tools/motion-bench, docs-fr/fiches-cloud/M-mouvement-ecran.md) can
// serve the same traffic in real time.
//
// Aircraft fly known paths -- straight, turning, climbing, slowing down,
// losing their position for a while -- and are decoded at the rates measured
// on the station on 13/10/2026: a position about 0.6 times a second and per
// aircraft, a velocity 1.5 times a second, so a receiver polled once a second
// repeats the previous fix with seen_pos one second older about half the
// time. Only the generator knows where each aircraft truly is (TruthAt).
// Nothing comes from the station: the place, the aircraft and their paths are
// made up.
package simtraffic

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

const (
	PosRate   = 0.6 // positions decoded per second and per aircraft (station, 13/10/2026)
	VelRate   = 1.5 // velocity and altitude messages per second
	PosNoiseM = 5.0 // standard deviation of a decoded position, metres
	KtToMps   = 0.514444

	stepSec = 0.01 // integration step of the true paths
)

// Flight is one synthetic aircraft: where it starts and the controls it
// flies, each a function of the seconds since the start.
type Flight struct {
	Hex, Callsign, Kind string
	Lat, Lon, AltFt     float64
	GsKt, TrackDeg      float64
	DriftDeg            float64                 // true heading minus track (wind)
	Turn                func(t float64) float64 // deg/s
	Accel               func(t float64) float64 // kt/s
	VRate               func(t float64) float64 // ft/min
	GapFrom, GapTo      float64                 // seconds without a decoded position, 0 0 for none
	GapDropsPosition    bool                    // during the gap the source omits lat/lon instead of repeating them
}

// Constant is a control that never changes.
func Constant(v float64) func(float64) float64 { return func(float64) float64 { return v } }

// Between is a control worth v from `from` to `to` seconds, 0 otherwise.
func Between(from, to, v float64) func(float64) float64 {
	return func(t float64) float64 {
		if t >= from && t < to {
			return v
		}
		return 0
	}
}

// Fleet12 is the replay bench's fleet (fiche L), around a receiver at
// 45.0 / 4.67: its reference metrics were measured on it, so it never changes.
func Fleet12() []Flight {
	return []Flight{
		{Hex: "f00001", Callsign: "TST101", Kind: "cruise", Lat: 44.80, Lon: 4.20, AltFt: 35000, GsKt: 450, TrackDeg: 60, DriftDeg: 6,
			Turn: Constant(0), Accel: Constant(0), VRate: Constant(0)},
		{Hex: "f00002", Callsign: "TST102", Kind: "cruise", Lat: 45.30, Lon: 5.10, AltFt: 37000, GsKt: 420, TrackDeg: 225, DriftDeg: -4,
			Turn: Constant(0), Accel: Constant(0), VRate: Constant(0)},
		{Hex: "f00003", Callsign: "TST103", Kind: "turn", Lat: 45.05, Lon: 4.75, AltFt: 8000, GsKt: 250, TrackDeg: 0, DriftDeg: 3,
			Turn: Constant(3), Accel: Constant(0), VRate: Constant(0)},
		{Hex: "f00004", Callsign: "TST104", Kind: "turn", Lat: 44.95, Lon: 4.50, AltFt: 12000, GsKt: 300, TrackDeg: 300, DriftDeg: 2,
			Turn: Constant(-1), Accel: Constant(0), VRate: Constant(0)},
		{Hex: "f00005", Callsign: "TST105", Kind: "climb", Lat: 45.00, Lon: 4.68, AltFt: 3000, GsKt: 220, TrackDeg: 110, DriftDeg: 1,
			Turn: Constant(0), Accel: Constant(0.5), VRate: Constant(2500)},
		{Hex: "f00006", Callsign: "TST106", Kind: "descent", Lat: 45.25, Lon: 4.40, AltFt: 9000, GsKt: 280, TrackDeg: 150, DriftDeg: -2,
			Turn: Constant(0), Accel: Constant(-0.4), VRate: Constant(-1500)},
		{Hex: "f00007", Callsign: "TST107", Kind: "slowdown", Lat: 45.10, Lon: 4.95, AltFt: 4000, GsKt: 190, TrackDeg: 260, DriftDeg: 0,
			Turn: Constant(0), Accel: Between(20, 70, -1), VRate: Constant(-700)},
		{Hex: "f00008", Callsign: "TST108", Kind: "turn", Lat: 44.85, Lon: 4.85, AltFt: 15000, GsKt: 320, TrackDeg: 20, DriftDeg: 4,
			Turn: Between(60, 105, 2), Accel: Constant(0), VRate: Constant(0)},
		{Hex: "f00009", Callsign: "TST109", Kind: "gap", Lat: 44.90, Lon: 4.30, AltFt: 30000, GsKt: 400, TrackDeg: 80, DriftDeg: 5,
			Turn: Constant(0), Accel: Constant(0), VRate: Constant(0), GapFrom: 60, GapTo: 90},
		{Hex: "f0000a", Callsign: "TST110", Kind: "gap", Lat: 45.20, Lon: 4.90, AltFt: 11000, GsKt: 280, TrackDeg: 190, DriftDeg: -3,
			Turn: Constant(1.5), Accel: Constant(0), VRate: Constant(0), GapFrom: 100, GapTo: 130},
		{Hex: "f0000b", Callsign: "TST111", Kind: "gap", Lat: 44.75, Lon: 4.70, AltFt: 24000, GsKt: 350, TrackDeg: 10, DriftDeg: 2,
			Turn: Constant(0), Accel: Constant(0), VRate: Constant(0), GapFrom: 70, GapTo: 100, GapDropsPosition: true},
		{Hex: "f0000c", Callsign: "TST112", Kind: "slow", Lat: 45.02, Lon: 4.60, AltFt: 1500, GsKt: 110, TrackDeg: 330, DriftDeg: 8,
			Turn: Between(120, 150, -2), Accel: Constant(0), VRate: Constant(0)},
	}
}

// Options of a traffic.
type Options struct {
	// Aircraft flown: the first twelve are Fleet12, the others are drawn
	// around the station (Generated). 0 means 12.
	Aircraft int
	// Seconds of flight, from 0.
	Seconds float64
	// Seed of every random draw: decoding instants, noise, generated fleet.
	Seed int64
	// The receiver the generated aircraft fly around.
	StationLat, StationLon float64
	// The long gap (fiche M, the AIC143 case: 18 s without a position at
	// 45 NM, then a fix again), flown by the first generated aircraft, in
	// seconds from the start. LongGapLen 0 means 18 s.
	LongGapFrom, LongGapLen float64
	// One true state in StoreEvery is kept (TruthAt interpolates between
	// them); 0 or 1 keeps every 10 ms state, as the replay bench does. A long
	// run of 150 aircraft needs 10 (100 ms) to stay within a few hundred MB.
	StoreEvery int
}

// State is an aircraft's true state at an instant.
type State struct {
	T                        float64 // seconds since the start
	Lat, Lon, AltFt          float64
	GsKt, TrackDeg, VRateFPM float64
	TrueHeadingDeg           float64
}

// Aircraft is a flight flown: its true path and when the receiver decodes it.
type Aircraft struct {
	Flight
	path       []State
	storeEvery int
	posAt      []float64
	velAt      []float64
	noise      map[float64][2]float64 // the noise of each decoded position, drawn once
}

// Traffic is a fleet flown over Options.Seconds.
type Traffic struct {
	Fleet []*Aircraft
	opts  Options
}

// New flies the fleet. The draws are made in the order of the replay bench
// (fiche L): its twelve aircraft over 240 s give the same decodings, noise
// and paths as before the generator left its test file.
func New(o Options) *Traffic {
	if o.Aircraft <= 0 {
		o.Aircraft = 12
	}
	if o.StoreEvery < 1 {
		o.StoreEvery = 1
	}
	if o.LongGapLen <= 0 {
		o.LongGapLen = 18
	}
	flights := Fleet12()
	if o.Aircraft < len(flights) {
		flights = flights[:o.Aircraft]
	}
	if o.Aircraft > len(flights) {
		flights = append(flights, Generated(o, o.Aircraft-len(flights))...)
	}
	rng := rand.New(rand.NewSource(o.Seed))
	tr := &Traffic{opts: o}
	for _, f := range flights {
		a := &Aircraft{Flight: f, path: flyTruth(f, o.Seconds, o.StoreEvery), storeEvery: o.StoreEvery, noise: map[float64][2]float64{}}
		a.posAt = poissonTimes(rng, PosRate, o.Seconds)
		a.velAt = poissonTimes(rng, VelRate, o.Seconds)
		for _, pt := range a.posAt {
			a.noise[pt] = [2]float64{rng.NormFloat64() * PosNoiseM, rng.NormFloat64() * PosNoiseM}
		}
		tr.Fleet = append(tr.Fleet, a)
	}
	return tr
}

// Generated draws n more aircraft within 70 NM of the station, flying across
// it rather than away: cruising, turning, climbing, descending, slowing down,
// and one in ten losing its position for 30 s. The first one flies the long
// gap at 45 NM (Options.LongGapFrom).
func Generated(o Options, n int) []Flight {
	rng := rand.New(rand.NewSource(o.Seed + 1))
	kinds := []string{"cruise", "cruise", "turn", "climb", "descent", "cruise", "slowdown", "turn", "cruise", "gap"}
	out := make([]Flight, 0, n)
	for i := 0; i < n; i++ {
		kind := kinds[i%len(kinds)]
		distNm := 5 + rng.Float64()*65
		if i == 0 {
			kind, distNm = "longgap", 45
		}
		bearing := rng.Float64() * 360
		lat, lon := offset(o.StationLat, o.StationLon, bearing, distNm*1852)
		// Toward the station, give or take 80 degrees.
		track := math.Mod(bearing+180+(rng.Float64()*160-80)+360, 360)
		f := Flight{
			Hex: fmt.Sprintf("f1%04x", i), Callsign: fmt.Sprintf("SIM%03d", i), Kind: kind,
			Lat: lat, Lon: lon, TrackDeg: track, DriftDeg: rng.Float64()*16 - 8,
			Turn: Constant(0), Accel: Constant(0), VRate: Constant(0),
		}
		switch kind {
		case "cruise", "longgap":
			f.AltFt, f.GsKt = 25000+rng.Float64()*14000, 380+rng.Float64()*100
		case "gap":
			f.AltFt, f.GsKt = 20000+rng.Float64()*15000, 350+rng.Float64()*100
			f.GapFrom = 30 + rng.Float64()*math.Max(1, o.Seconds-90)
			f.GapTo = f.GapFrom + 30
		case "turn":
			f.AltFt, f.GsKt = 6000+rng.Float64()*10000, 240+rng.Float64()*80
			rate := (1 + rng.Float64()*2) * float64(1-2*rng.Intn(2))
			from := rng.Float64() * math.Max(1, o.Seconds-60)
			f.Turn = Between(from, from+30+rng.Float64()*60, rate)
		case "climb":
			f.AltFt, f.GsKt = 3000+rng.Float64()*5000, 200+rng.Float64()*60
			f.Accel, f.VRate = Constant(0.4), Constant(1500+rng.Float64()*1500)
		case "descent":
			f.AltFt, f.GsKt = 9000+rng.Float64()*8000, 260+rng.Float64()*60
			f.Accel, f.VRate = Constant(-0.3), Constant(-(1000 + rng.Float64()*1000))
		case "slowdown":
			f.AltFt, f.GsKt = 3000+rng.Float64()*3000, 180+rng.Float64()*40
			from := rng.Float64() * math.Max(1, o.Seconds-60)
			f.Accel, f.VRate = Between(from, from+40, -1), Constant(-700)
		}
		if kind == "longgap" {
			f.GapFrom, f.GapTo = o.LongGapFrom, o.LongGapFrom+o.LongGapLen
			if o.LongGapFrom <= 0 {
				f.GapFrom, f.GapTo = 0, 0
			}
		}
		out = append(out, f)
	}
	return out
}

func offset(lat, lon, bearingDeg, metres float64) (float64, float64) {
	b := bearingDeg * math.Pi / 180
	dLat := metres * math.Cos(b) / 111320
	dLon := metres * math.Sin(b) / (111320 * math.Cos(lat*math.Pi/180))
	return lat + dLat, lon + dLon
}

// flyTruth integrates a flight in 10 ms steps, keeping one state in
// storeEvery.
func flyTruth(f Flight, seconds float64, storeEvery int) []State {
	s := State{Lat: f.Lat, Lon: f.Lon, AltFt: f.AltFt, GsKt: f.GsKt, TrackDeg: f.TrackDeg}
	n := int(seconds/stepSec) + 1
	out := make([]State, 0, n/storeEvery+2)
	for i := 0; i < n; i++ {
		s.T = float64(i) * stepSec
		s.VRateFPM = f.VRate(s.T)
		s.TrueHeadingDeg = math.Mod(s.TrackDeg+f.DriftDeg+360, 360)
		if i%storeEvery == 0 {
			out = append(out, s)
		}
		v := s.GsKt * KtToMps
		tr := s.TrackDeg * math.Pi / 180
		s.Lat += v * math.Cos(tr) * stepSec / 111320
		s.Lon += v * math.Sin(tr) * stepSec / (111320 * math.Cos(s.Lat*math.Pi/180))
		s.AltFt += s.VRateFPM / 60 * stepSec
		s.GsKt = math.Max(60, s.GsKt+f.Accel(s.T)*stepSec)
		s.TrackDeg = math.Mod(s.TrackDeg+f.Turn(s.T)*stepSec+360, 360)
	}
	return out
}

// TruthAt is the aircraft's true state at t seconds, on the 10 ms grid.
func (a *Aircraft) TruthAt(t float64) State {
	i := int(math.Round(t / stepSec))
	if a.storeEvery == 1 {
		return a.path[clamp(i, len(a.path)-1)]
	}
	j := i / a.storeEvery
	if j >= len(a.path)-1 {
		return a.path[clamp(j, len(a.path)-1)]
	}
	if j < 0 {
		return a.path[0]
	}
	p, q := a.path[j], a.path[j+1]
	f := float64(i-j*a.storeEvery) / float64(a.storeEvery)
	dTrack := q.TrackDeg - p.TrackDeg
	if dTrack > 180 {
		dTrack -= 360
	} else if dTrack < -180 {
		dTrack += 360
	}
	lerp := func(x, y float64) float64 { return x + (y-x)*f }
	return State{
		T: float64(i) * stepSec, Lat: lerp(p.Lat, q.Lat), Lon: lerp(p.Lon, q.Lon), AltFt: lerp(p.AltFt, q.AltFt),
		GsKt: lerp(p.GsKt, q.GsKt), TrackDeg: math.Mod(p.TrackDeg+dTrack*f+360, 360), VRateFPM: p.VRateFPM,
		TrueHeadingDeg: math.Mod(p.TrackDeg+dTrack*f+a.DriftDeg+360, 360),
	}
}

func clamp(i, max int) int {
	if i < 0 {
		return 0
	}
	if i > max {
		return max
	}
	return i
}

// poissonTimes draws the instants of a process of the given rate over the
// run, from a little before its start so the first poll has a fix.
func poissonTimes(rng *rand.Rand, rate, seconds float64) []float64 {
	var out []float64
	for t := -2 + rng.ExpFloat64()/rate; t < seconds; t += rng.ExpFloat64() / rate {
		out = append(out, t)
	}
	return out
}

func lastBefore(times []float64, t float64) (float64, bool) {
	i := sort.SearchFloat64s(times, t)
	if i == 0 {
		return 0, false
	}
	return times[i-1], true
}

// Round rounds v to the given number of decimals.
func Round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}

// Target is the aircraft as aircraft.json shows it at file time t: the last
// decoded position and its age, the last velocity, readsb's precisions; nil
// before its first velocity message.
func (a *Aircraft) Target(t float64) map[string]any {
	velT, ok := lastBefore(a.velAt, t)
	if !ok {
		return nil
	}
	vel := a.TruthAt(math.Max(0, velT))
	alt := math.Round(vel.AltFt/25) * 25
	out := map[string]any{
		"hex": a.Hex, "type": "adsb_icao", "flight": a.Callsign + "  ", "category": "A3",
		"alt_baro": alt, "alt_geom": alt + 200,
		"gs": Round(vel.GsKt, 1), "track": Round(vel.TrackDeg, 2), "true_heading": Round(vel.TrueHeadingDeg, 2),
		"mag_heading": Round(math.Mod(vel.TrueHeadingDeg-2+360, 360), 2),
		"baro_rate":   math.Round(vel.VRateFPM/64) * 64,
		"squawk":      "1000", "messages": int(t*8) + 100, "seen": Round(t-velT, 1), "rssi": -20.5,
	}
	posT, ok := lastBefore(a.posAt, t)
	inGap := posT >= a.GapFrom && posT < a.GapTo && a.GapTo > 0
	if inGap {
		// No position decoded during the gap: the last one before it stands.
		posT, ok = lastBefore(a.posAt, a.GapFrom)
		if a.GapDropsPosition && t >= a.GapFrom && t < a.GapTo {
			return out
		}
	}
	if !ok {
		return out
	}
	p := a.TruthAt(math.Max(0, posT))
	n := a.noise[posT]
	out["lat"] = Round(p.Lat+n[0]/111320, 6)
	out["lon"] = Round(p.Lon+n[1]/(111320*math.Cos(p.Lat*math.Pi/180)), 6)
	out["seen_pos"] = Round(t-posT, 1)
	return out
}

// AircraftJSON is readsb's aircraft.json at file time t, its `now` counted
// from start.
func (tr *Traffic) AircraftJSON(start time.Time, t float64, messages int) ([]byte, error) {
	var targets []map[string]any
	for _, a := range tr.Fleet {
		if tg := a.Target(t); tg != nil {
			targets = append(targets, tg)
		}
	}
	now := float64(start.Unix()) + float64(start.Nanosecond())/1e9 + t
	return json.Marshal(map[string]any{"now": now, "messages": messages, "aircraft": targets})
}
