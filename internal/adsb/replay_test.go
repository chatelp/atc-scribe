package adsb

// The replay bench (docs-fr/fiches-cloud/L-bloc-position.md, step 1).
//
// Synthetic aircraft flown on known paths -- straight, turning, climbing,
// slowing down, losing their position for thirty seconds -- are written to a
// readsb aircraft.json the way the station answers: polled once a second, a
// new position at about 0.6 per second and per aircraft (the rate measured on
// the station, 13/10/2026), so about half the polls repeat the previous fix
// with seen_pos one second older. The server reads that file through its own
// chain (readsb-file client, fetchAndProcess, change detector, trajectory
// tracker, live prediction) on a simulated clock, and every WebSocket message
// it broadcasts is recorded with the time it left.
//
// The recording and the true paths go to testdata/replay/, where
// www/aircraft-replay.test.js plays them through the page's feed and its
// animation engine and measures the motion drawn against the truth, which only
// the generator knows. Nothing here comes from the station: the place, the
// aircraft and their paths are made up.
//
// The recording is rewritten only on demand:
//
//	go test ./internal/adsb -run TestReplayBench -update-replay
//
// Without the flag the test records into memory and checks the stream is
// whole; it does not compare bytes, since floating point may differ by a last
// digit between machines.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

var updateReplay = flag.Bool("update-replay", false, "rewrite testdata/replay from the generator")

const (
	replayStationLat = 45.0 // a made-up receiver, the place the other tests use
	replayStationLon = 4.67
	replaySeconds    = 240
	replayPosRate    = 0.6 // positions decoded per second and per aircraft (station, 13/10/2026)
	replayVelRate    = 1.5 // velocity and altitude messages per second
	replayFileAge    = 500 * time.Millisecond
	replayPosNoiseM  = 5.0 // standard deviation of a decoded position, metres
	replayTruthStep  = 200 * time.Millisecond
	replayDeliveryMs = 20 // from broadcast to the page
	replayKtToMps    = 0.514444
)

var replayStart = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// replayFlight is one synthetic aircraft: where it starts and the controls it
// flies, each a function of the seconds since the start of the replay.
type replayFlight struct {
	hex, flight, kind string
	lat, lon, altFt   float64
	gsKt, trackDeg    float64
	driftDeg          float64                 // true heading minus track (wind)
	turn              func(t float64) float64 // deg/s
	accel             func(t float64) float64 // kt/s
	vrate             func(t float64) float64 // ft/min
	gapFrom, gapTo    float64                 // seconds without a decoded position, 0 0 for none
	gapDropsPosition  bool                    // during the gap the source omits lat/lon instead of repeating them
}

func constant(v float64) func(float64) float64 { return func(float64) float64 { return v } }

func between(from, to, v float64) func(float64) float64 {
	return func(t float64) float64 {
		if t >= from && t < to {
			return v
		}
		return 0
	}
}

func replayFlights() []replayFlight {
	return []replayFlight{
		{hex: "f00001", flight: "TST101", kind: "cruise", lat: 44.80, lon: 4.20, altFt: 35000, gsKt: 450, trackDeg: 60, driftDeg: 6,
			turn: constant(0), accel: constant(0), vrate: constant(0)},
		{hex: "f00002", flight: "TST102", kind: "cruise", lat: 45.30, lon: 5.10, altFt: 37000, gsKt: 420, trackDeg: 225, driftDeg: -4,
			turn: constant(0), accel: constant(0), vrate: constant(0)},
		{hex: "f00003", flight: "TST103", kind: "turn", lat: 45.05, lon: 4.75, altFt: 8000, gsKt: 250, trackDeg: 0, driftDeg: 3,
			turn: constant(3), accel: constant(0), vrate: constant(0)},
		{hex: "f00004", flight: "TST104", kind: "turn", lat: 44.95, lon: 4.50, altFt: 12000, gsKt: 300, trackDeg: 300, driftDeg: 2,
			turn: constant(-1), accel: constant(0), vrate: constant(0)},
		{hex: "f00005", flight: "TST105", kind: "climb", lat: 45.00, lon: 4.68, altFt: 3000, gsKt: 220, trackDeg: 110, driftDeg: 1,
			turn: constant(0), accel: constant(0.5), vrate: constant(2500)},
		{hex: "f00006", flight: "TST106", kind: "descent", lat: 45.25, lon: 4.40, altFt: 9000, gsKt: 280, trackDeg: 150, driftDeg: -2,
			turn: constant(0), accel: constant(-0.4), vrate: constant(-1500)},
		{hex: "f00007", flight: "TST107", kind: "slowdown", lat: 45.10, lon: 4.95, altFt: 4000, gsKt: 190, trackDeg: 260, driftDeg: 0,
			turn: constant(0), accel: between(20, 70, -1), vrate: constant(-700)},
		{hex: "f00008", flight: "TST108", kind: "turn", lat: 44.85, lon: 4.85, altFt: 15000, gsKt: 320, trackDeg: 20, driftDeg: 4,
			turn: between(60, 105, 2), accel: constant(0), vrate: constant(0)},
		{hex: "f00009", flight: "TST109", kind: "gap", lat: 44.90, lon: 4.30, altFt: 30000, gsKt: 400, trackDeg: 80, driftDeg: 5,
			turn: constant(0), accel: constant(0), vrate: constant(0), gapFrom: 60, gapTo: 90},
		{hex: "f0000a", flight: "TST110", kind: "gap", lat: 45.20, lon: 4.90, altFt: 11000, gsKt: 280, trackDeg: 190, driftDeg: -3,
			turn: constant(1.5), accel: constant(0), vrate: constant(0), gapFrom: 100, gapTo: 130},
		{hex: "f0000b", flight: "TST111", kind: "gap", lat: 44.75, lon: 4.70, altFt: 24000, gsKt: 350, trackDeg: 10, driftDeg: 2,
			turn: constant(0), accel: constant(0), vrate: constant(0), gapFrom: 70, gapTo: 100, gapDropsPosition: true},
		{hex: "f0000c", flight: "TST112", kind: "slow", lat: 45.02, lon: 4.60, altFt: 1500, gsKt: 110, trackDeg: 330, driftDeg: 8,
			turn: between(120, 150, -2), accel: constant(0), vrate: constant(0)},
	}
}

// replayState is an aircraft's true state at an instant.
type replayState struct {
	t                        float64 // seconds since the start
	lat, lon, altFt          float64
	gsKt, trackDeg, vrateFPM float64
	trueHeadingDeg           float64
}

// flyTruth integrates a flight in 10 ms steps over the whole replay.
func flyTruth(f replayFlight) []replayState {
	const dt = 0.01
	s := replayState{lat: f.lat, lon: f.lon, altFt: f.altFt, gsKt: f.gsKt, trackDeg: f.trackDeg}
	n := int(replaySeconds/dt) + 1
	out := make([]replayState, 0, n)
	for i := 0; i < n; i++ {
		s.t = float64(i) * dt
		s.vrateFPM = f.vrate(s.t)
		s.trueHeadingDeg = math.Mod(s.trackDeg+f.driftDeg+360, 360)
		out = append(out, s)
		v := s.gsKt * replayKtToMps
		tr := s.trackDeg * math.Pi / 180
		s.lat += v * math.Cos(tr) * dt / 111320
		s.lon += v * math.Sin(tr) * dt / (111320 * math.Cos(s.lat*math.Pi/180))
		s.altFt += s.vrateFPM / 60 * dt
		s.gsKt = math.Max(60, s.gsKt+f.accel(s.t)*dt)
		s.trackDeg = math.Mod(s.trackDeg+f.turn(s.t)*dt+360, 360)
	}
	return out
}

func truthAt(path []replayState, t float64) replayState {
	i := int(math.Round(t / 0.01))
	if i < 0 {
		i = 0
	}
	if i >= len(path) {
		i = len(path) - 1
	}
	return path[i]
}

// poissonTimes draws the instants of a process of the given rate over the
// replay, from a little before its start so the first poll has a fix.
func poissonTimes(rng *rand.Rand, rate float64) []float64 {
	var out []float64
	for t := -2 + rng.ExpFloat64()/rate; t < replaySeconds; t += rng.ExpFloat64() / rate {
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

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}

type replayAircraft struct {
	f      replayFlight
	path   []replayState
	posAt  []float64
	velAt  []float64
	noiseN map[float64][2]float64 // the noise of each decoded position, drawn once
}

// target is the aircraft as aircraft.json shows it at file time t: the last
// decoded position and its age, the last velocity, readsb's precisions.
func (a *replayAircraft) target(t float64) map[string]any {
	velT, ok := lastBefore(a.velAt, t)
	if !ok {
		return nil
	}
	vel := truthAt(a.path, math.Max(0, velT))
	alt := math.Round(vel.altFt/25) * 25
	out := map[string]any{
		"hex": a.f.hex, "type": "adsb_icao", "flight": a.f.flight + "  ", "category": "A3",
		"alt_baro": alt, "alt_geom": alt + 200,
		"gs": round(vel.gsKt, 1), "track": round(vel.trackDeg, 2), "true_heading": round(vel.trueHeadingDeg, 2),
		"mag_heading": round(math.Mod(vel.trueHeadingDeg-2+360, 360), 2),
		"baro_rate":   math.Round(vel.vrateFPM/64) * 64,
		"squawk":      "1000", "messages": int(t*8) + 100, "seen": round(t-velT, 1), "rssi": -20.5,
	}
	posT, ok := lastBefore(a.posAt, t)
	inGap := posT >= a.f.gapFrom && posT < a.f.gapTo && a.f.gapTo > 0
	if inGap {
		// No position decoded during the gap: the last one before it stands.
		posT, ok = lastBefore(a.posAt, a.f.gapFrom)
		if a.f.gapDropsPosition && t >= a.f.gapFrom && t < a.f.gapTo {
			return out
		}
	}
	if !ok {
		return out
	}
	p := truthAt(a.path, math.Max(0, posT))
	n := a.noiseN[posT]
	out["lat"] = round(p.lat+n[0]/111320, 6)
	out["lon"] = round(p.lon+n[1]/(111320*math.Cos(p.lat*math.Pi/180)), 6)
	out["seen_pos"] = round(t-posT, 1)
	return out
}

// replayStorage keeps what the chain writes and answers its batch reads, the
// minimum fetchAndProcess needs; anything else panics.
type replayStorage struct {
	Storage
	aircraft map[string]*Aircraft
	phases   map[string]*PhaseChange
}

func (s *replayStorage) GetAircraftOnGroundBatch(hexes []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, h := range hexes {
		if a, ok := s.aircraft[h]; ok {
			out[h] = a.OnGround
		}
	}
	return out, nil
}

func (s *replayStorage) GetLatestADSBDataBatch(hexes []string) (map[string]*ADSBTarget, error) {
	out := map[string]*ADSBTarget{}
	for _, h := range hexes {
		if a, ok := s.aircraft[h]; ok {
			out[h] = a.ADSB
		}
	}
	return out, nil
}

func (s *replayStorage) GetLatestTakeoffTimesBatch([]string) (map[string]*time.Time, error) {
	return map[string]*time.Time{}, nil
}

func (s *replayStorage) GetLatestLandingTimesBatch([]string) (map[string]*time.Time, error) {
	return map[string]*time.Time{}, nil
}

func (s *replayStorage) GetCurrentPhasesBatch(hexes []string) (map[string]*PhaseChange, error) {
	out := map[string]*PhaseChange{}
	for _, h := range hexes {
		if p, ok := s.phases[h]; ok {
			out[h] = p
		}
	}
	return out, nil
}

func (s *replayStorage) GetLatestADSBTargetIDsBatch([]string) (map[string]*int, error) {
	return map[string]*int{}, nil
}

func (s *replayStorage) InsertPhaseChangesBatch(changes []PhaseChangeInsert) error {
	for _, c := range changes {
		s.phases[c.Hex] = &PhaseChange{Phase: c.Phase, Timestamp: c.Timestamp, Airport: c.Airport}
	}
	return nil
}

func (s *replayStorage) Upsert(a *Aircraft) { s.aircraft[a.Hex] = a }
func (s *replayStorage) Count() int         { return len(s.aircraft) }

func (s *replayStorage) GetStaleActiveAircraft([]string, time.Time) ([]*Aircraft, error) {
	return nil, nil
}

// replayRecorder is the page's end of the WebSocket: each message as the
// browser would receive it, with the simulated time it left the server.
type replayRecorder struct {
	now  func() time.Time
	msgs []replayMessage
	// forms holds, by message type, every key path the messages carried
	// before anything was left out of the recording (message_form_test.go).
	forms map[string]map[string]bool
}

type replayMessage struct {
	AtMs int64           `json:"at_ms"` // when the page receives it, ms since the start of the replay
	Msg  json.RawMessage `json:"msg"`
}

func (r *replayRecorder) Broadcast(m *websocket.Message) {
	switch m.Type {
	case websocket.MessageTypeAircraftAdded, websocket.MessageTypeAircraftUpdate,
		websocket.MessageTypeAircraftRemoved, websocket.MessageTypeAircraftPredictedState:
	default:
		return // phases, status: not the motion
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	// The predicted positions each aircraft object carries (future, hindcast)
	// are left out of the recording, three quarters of its size: the page reads
	// them from /tracks, never from the stream.
	var generic map[string]any
	if err := json.Unmarshal(b, &generic); err != nil {
		panic(err)
	}
	if r.forms[m.Type] == nil {
		r.forms[m.Type] = map[string]bool{}
	}
	keyPaths("", generic, r.forms[m.Type])
	if data, ok := generic["data"].(map[string]any); ok {
		if a, ok := data["aircraft"].(map[string]any); ok {
			delete(a, "future")
			delete(a, "hindcast")
		}
	}
	if b, err = json.Marshal(generic); err != nil {
		panic(err)
	}
	at := r.now().Sub(replayStart).Milliseconds() + replayDeliveryMs
	r.msgs = append(r.msgs, replayMessage{AtMs: at, Msg: b})
}

type replayTruth struct {
	Hex    string       `json:"hex"`
	Flight string       `json:"flight"`
	Kind   string       `json:"kind"`
	GapMs  [2]int64     `json:"gap_ms"`
	StepMs int64        `json:"step_ms"`
	Points [][5]float64 `json:"points"` // lat, lon, alt ft, track deg, ground speed m/s, every StepMs from 0
}

type replayRecording struct {
	Forms    map[string]map[string]bool `json:"-"`
	Start    string                     `json:"start"`
	Station  [2]float64                 `json:"station"`
	Messages []replayMessage            `json:"messages"`
	Truth    []replayTruth              `json:"truth"`
}

// recordReplay flies the aircraft and runs the server's chain over them.
func recordReplay(t *testing.T) replayRecording {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, f := range []string{"receiver.json", "stats.json"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rng := rand.New(rand.NewSource(20261013))
	var fleet []*replayAircraft
	for _, f := range replayFlights() {
		a := &replayAircraft{f: f, path: flyTruth(f), noiseN: map[float64][2]float64{}}
		a.posAt = poissonTimes(rng, replayPosRate)
		a.velAt = poissonTimes(rng, replayVelRate)
		for _, pt := range a.posAt {
			a.noiseN[pt] = [2]float64{rng.NormFloat64() * replayPosNoiseM, rng.NormFloat64() * replayPosNoiseM}
		}
		fleet = append(fleet, a)
	}

	var simNow time.Time
	clock := func() time.Time { return simNow }
	rec := &replayRecorder{now: clock, forms: map[string]map[string]bool{}}
	phases := config.FlightPhasesConfig{
		Enabled: true, CruiseAltitudeFt: 18000, DepartureAltitudeFt: 1000,
		TaxiingMinSpeedKts: 1, TaxiingMaxSpeedKts: 50,
		ApproachCenterlineToleranceNM: 0.5, ApproachMaxDistanceNM: 10,
		ApproachMaxAltitudeFt: 5000, ApproachHeadingToleranceDeg: 30,
		RecentTakeoffTimeoutMinutes: 30, AirportRangeNM: 5,
		FlyingMinTASKts: 50, FlyingMinAltFt: 300,
		RunwayInUseWindowMinutes: 60, RunwayInUseApproachWeight: 2,
		RunwayInUseLandingWeight: 3, RunwayInUseClimbWeight: 2, RunwayInUseDecayRate: 0.98,
		TrajectoryBufferDurationSec: 90, TrajectoryMinPoints: 5,
		// The tracker's cleanup runs on the wall clock; the simulated one is
		// elsewhere in time, so it must not run during the replay.
		TrajectoryStaleTimeoutSec: 1 << 20, TrajectoryCleanupIntervalSec: 3600,
		TrajectoryDescentThresholdFPM: -200, TrajectoryClimbThresholdFPM: 200,
		TrajectoryLevelBandFt: 200, TrajectoryTurningRateDeg: 1.5,
	}
	adsbCfg := config.ADSBConfig{SourceType: "readsb-file", ReadsbDataDir: dir, FetchIntervalSecs: 1, SignalLostTimeoutSecs: 60}
	station := config.StationConfig{Latitude: replayStationLat, Longitude: replayStationLon, AirportCode: "TEST"}
	client := NewClient(adsbCfg, station.Latitude, station.Longitude, time.Second, log)
	store := &replayStorage{aircraft: map[string]*Aircraft{}, phases: map[string]*PhaseChange{}}
	s := NewService(client, store, time.Second, log, station, adsbCfg, phases, rec)
	s.clock = clock
	t.Cleanup(s.trajectoryTracker.Stop)

	// readsb writes aircraft.json once a second; the server reads it half a
	// second later (replayFileAge); the prediction ticker runs on its own
	// second, here half-way between two polls.
	ctx := context.Background()
	for k := 0; k < replaySeconds; k++ {
		fileT := float64(k)
		var targets []map[string]any
		for _, a := range fleet {
			if tg := a.target(fileT); tg != nil {
				targets = append(targets, tg)
			}
		}
		body, err := json.Marshal(map[string]any{"now": float64(replayStart.Unix()) + fileT, "messages": k * 1000, "aircraft": targets})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "aircraft.json"), body, 0o644); err != nil {
			t.Fatal(err)
		}

		simNow = replayStart.Add(time.Duration(k)*time.Second + replayFileAge)
		start := len(rec.msgs)
		if err := s.fetchAndProcess(ctx); err != nil {
			t.Fatalf("poll %d: %v", k, err)
		}
		sortReplayMessages(rec.msgs[start:])

		simNow = replayStart.Add(time.Duration(k)*time.Second + replayFileAge + 500*time.Millisecond)
		start = len(rec.msgs)
		s.live.broadcast(simNow)
		sortReplayMessages(rec.msgs[start:])
	}

	out := replayRecording{
		Start:    replayStart.Format(time.RFC3339),
		Station:  [2]float64{replayStationLat, replayStationLon},
		Messages: rec.msgs,
		Forms:    rec.forms,
	}
	for _, a := range fleet {
		tr := replayTruth{Hex: a.f.hex, Flight: a.f.flight, Kind: a.f.kind, StepMs: replayTruthStep.Milliseconds(),
			GapMs: [2]int64{int64(a.f.gapFrom * 1000), int64(a.f.gapTo * 1000)}}
		for ms := int64(0); ms <= replaySeconds*1000; ms += tr.StepMs {
			p := truthAt(a.path, float64(ms)/1000)
			tr.Points = append(tr.Points, [5]float64{round(p.lat, 7), round(p.lon, 7), round(p.altFt, 0), round(p.trackDeg, 2), round(p.gsKt*replayKtToMps, 2)})
		}
		out.Truth = append(out.Truth, tr)
	}
	return out
}

// The server broadcasts the aircraft of one poll in map order; sorted, the
// recording is the same from one run to the next.
func sortReplayMessages(msgs []replayMessage) {
	sort.SliceStable(msgs, func(i, j int) bool { return string(msgs[i].Msg) < string(msgs[j].Msg) })
}

func TestReplayBench(t *testing.T) {
	rec := recordReplay(t)

	counts := map[string]int{}
	perAircraft := map[string]map[string]int{}
	for _, m := range rec.Messages {
		var msg struct {
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(m.Msg, &msg); err != nil {
			t.Fatal(err)
		}
		counts[msg.Type]++
		hex, _ := msg.Data["hex"].(string)
		if perAircraft[hex] == nil {
			perAircraft[hex] = map[string]int{}
		}
		perAircraft[hex][msg.Type]++
	}
	t.Logf("%d messages: %v", len(rec.Messages), counts)
	for _, f := range replayFlights() {
		c := perAircraft[f.hex]
		if c[websocket.MessageTypeAircraftAdded] != 1 {
			t.Errorf("%s: added %d times, want once", f.hex, c[websocket.MessageTypeAircraftAdded])
		}
		// About one real update a second (a poll that changes nothing sends
		// none) and a predicted state for most seconds.
		if c[websocket.MessageTypeAircraftUpdate] < replaySeconds*8/10 {
			t.Errorf("%s: %d updates in %d s", f.hex, c[websocket.MessageTypeAircraftUpdate], replaySeconds)
		}
		if c[websocket.MessageTypeAircraftPredictedState] < replaySeconds/2 {
			t.Errorf("%s: %d predicted states in %d s", f.hex, c[websocket.MessageTypeAircraftPredictedState], replaySeconds)
		}
	}

	if !*updateReplay {
		return
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.ModTime = time.Time{}
	enc := json.NewEncoder(zw)
	if err := enc.Encode(rec); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "replay", "recording.json.gz")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s, %d bytes", path, buf.Len())
}

func (s *replayStorage) GetByHex(hex string) (*Aircraft, bool) {
	a, ok := s.aircraft[hex]
	return a, ok
}

func (s *replayStorage) GetPhaseHistory(hex string) ([]PhaseChange, error) {
	if p, ok := s.phases[hex]; ok {
		return []PhaseChange{*p}, nil
	}
	return nil, nil
}
