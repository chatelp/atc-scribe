package adsb

// The replay bench (docs-fr/fiches-cloud/L-bloc-position.md, step 1).
//
// Synthetic aircraft flown on known paths -- straight, turning, climbing,
// slowing down, losing their position for thirty seconds -- are written to a
// readsb aircraft.json the way the station answers: polled once a second, a
// new position at about 0.6 per second and per aircraft (the rate measured on
// the station, 13/10/2026), so about half the polls repeat the previous fix
// with seen_pos one second older. The generator is internal/adsb/simtraffic,
// shared with the browser bench (tools/motion-bench). The server reads that file through its own
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
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/adsb/simtraffic"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

var updateReplay = flag.Bool("update-replay", false, "rewrite testdata/replay from the generator")

const (
	replayStationLat = 45.0 // a made-up receiver, the place the other tests use
	replayStationLon = 4.67
	replaySeconds    = 240
	replayFileAge    = 500 * time.Millisecond
	replayTruthStep  = 200 * time.Millisecond
	replayDeliveryMs = 20 // from broadcast to the page
)

var replayStart = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// replayTraffic is the fleet of the bench, flown by the generator the browser
// bench shares (internal/adsb/simtraffic): twelve aircraft over 240 s.
func replayTraffic() *simtraffic.Traffic {
	return simtraffic.New(simtraffic.Options{Aircraft: 12, Seconds: replaySeconds, Seed: 20261013,
		StationLat: replayStationLat, StationLon: replayStationLon})
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

	traffic := replayTraffic()

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
		body, err := traffic.AircraftJSON(replayStart, fileT, k*1000)
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
	for _, a := range traffic.Fleet {
		tr := replayTruth{Hex: a.Hex, Flight: a.Callsign, Kind: a.Kind, StepMs: replayTruthStep.Milliseconds(),
			GapMs: [2]int64{int64(a.GapFrom * 1000), int64(a.GapTo * 1000)}}
		for ms := int64(0); ms <= replaySeconds*1000; ms += tr.StepMs {
			p := a.TruthAt(float64(ms) / 1000)
			tr.Points = append(tr.Points, [5]float64{simtraffic.Round(p.Lat, 7), simtraffic.Round(p.Lon, 7), simtraffic.Round(p.AltFt, 0),
				simtraffic.Round(p.TrackDeg, 2), simtraffic.Round(p.GsKt*simtraffic.KtToMps, 2)})
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
	for _, f := range simtraffic.Fleet12() {
		c := perAircraft[f.Hex]
		if c[websocket.MessageTypeAircraftAdded] != 1 {
			t.Errorf("%s: added %d times, want once", f.Hex, c[websocket.MessageTypeAircraftAdded])
		}
		// About one real update a second (a poll that changes nothing sends
		// none) and a predicted state for most seconds.
		if c[websocket.MessageTypeAircraftUpdate] < replaySeconds*8/10 {
			t.Errorf("%s: %d updates in %d s", f.Hex, c[websocket.MessageTypeAircraftUpdate], replaySeconds)
		}
		if c[websocket.MessageTypeAircraftPredictedState] < replaySeconds/2 {
			t.Errorf("%s: %d predicted states in %d s", f.Hex, c[websocket.MessageTypeAircraftPredictedState], replaySeconds)
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
