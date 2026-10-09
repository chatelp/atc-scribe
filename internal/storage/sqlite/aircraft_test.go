package sqlite

import (
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
)

// t0 is a fixed instant, so that nothing below depends on the clock unless it
// says so. The few queries that read "the last hour" against time.Now() are
// fed instants relative to it, with half an hour of margin either side.
var t0 = time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)

func newAircraftStore(t *testing.T) *AircraftStorage {
	t.Helper()
	s, err := NewAircraftStorage(DailyPath(t.TempDir(), t0), testLog(t))
	if err != nil {
		t.Fatalf("NewAircraftStorage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func fp(v float64) *float64 { return &v }

// report is one ADS-B report with every column of the UNIQUE constraint set.
func report(hex string, lat, lon, alt float64) *adsb.ADSBTarget {
	return &adsb.ADSBTarget{
		Hex: hex, Flight: "AFR1081", Squawk: "1000",
		Lat: fp(lat), Lon: fp(lon), AltBaro: adsb.FlexibleFloat64(alt),
		GS: fp(250), TAS: fp(260), Track: fp(270),
	}
}

func seen(hex string, at time.Time, r *adsb.ADSBTarget) *adsb.Aircraft {
	return &adsb.Aircraft{Hex: hex, Flight: "AFR1081", Airline: "AFR", LastSeen: at, ADSB: r}
}

func positionsOf(t *testing.T, s *AircraftStorage, hex string) int {
	t.Helper()
	var n int
	if err := s.GetDB().QueryRow(`SELECT COUNT(*) FROM adsb_targets WHERE aircraft_hex = ?`, hex).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The receiver is polled every second, and an aircraft whose position has not
// changed is reported again with the same values: the UNIQUE constraint and
// INSERT OR IGNORE keep one row for it, and a new position adds one.
func TestARepeatedPositionIsStoredOnce(t *testing.T) {
	s := newAircraftStore(t)
	s.Upsert(seen("39c4a1", t0, report("39c4a1", 48.81, 2.07, 3000)))
	s.Upsert(seen("39c4a1", t0.Add(time.Second), report("39c4a1", 48.81, 2.07, 3000)))
	if n := positionsOf(t, s, "39c4a1"); n != 1 {
		t.Fatalf("the same report twice: %d rows, want 1", n)
	}
	s.Upsert(seen("39c4a1", t0.Add(2*time.Second), report("39c4a1", 48.82, 2.07, 3000)))
	if n := positionsOf(t, s, "39c4a1"); n != 2 {
		t.Errorf("a new position: %d rows, want 2", n)
	}
	if n := s.Count(); n != 1 {
		t.Errorf("one aircraft, %d rows in aircraft", n)
	}
}

// The full message lives on the aircraft row and is the latest one written; the
// source defaults to "local". An aircraft known without a message has no entry.
func TestTheLatestMessageIsReadFromTheAircraftRow(t *testing.T) {
	s := newAircraftStore(t)
	first := report("39c4a1", 48.81, 2.07, 3000)
	first.Registration, first.AircraftType = "F-GKXA", "A320"
	s.Upsert(seen("39c4a1", t0, first))
	second := report("39c4a1", 48.83, 2.07, 3500)
	second.Squawk = "7000"
	second.Registration, second.AircraftType = "F-GKXA", "A320"
	s.Upsert(seen("39c4a1", t0.Add(time.Second), second))
	s.Upsert(&adsb.Aircraft{Hex: "3c6444", LastSeen: t0})

	got, err := s.GetLatestADSBDataBatch([]string{"39c4a1", "3c6444", "absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("only the aircraft with a message should be returned: %v", got)
	}
	m := got["39c4a1"]
	if m == nil || m.Squawk != "7000" || adsb.NumberOrZero(m.Lat) != 48.83 {
		t.Fatalf("the latest message should be returned, got %+v", m)
	}
	if m.SourceType != "local" || m.Registration != "F-GKXA" || m.AircraftType != "A320" {
		t.Errorf("source %q, registration %q, type %q", m.SourceType, m.Registration, m.AircraftType)
	}
	if n := positionsOf(t, s, "3c6444"); n != 0 {
		t.Errorf("an aircraft without a message has no position, got %d rows", n)
	}
	if empty, err := s.GetLatestADSBDataBatch(nil); err != nil || len(empty) != 0 {
		t.Errorf("no hex: %v, %v", empty, err)
	}
}

// An update replaces the aircraft's state, and last_seen is stored in UTC
// whatever zone it was given in: the queries compare it as text.
func TestAnUpdateReplacesTheAircraftState(t *testing.T) {
	s := newAircraftStore(t)
	paris := time.FixedZone("CEST", 2*3600)
	s.Upsert(seen("39c4a1", t0.In(paris), report("39c4a1", 48.81, 2.07, 3000)))

	a, ok := s.GetByHex("39c4a1")
	if !ok {
		t.Fatal("the aircraft should be found")
	}
	if a.Status != "active" {
		t.Errorf("a new aircraft is active, got %q", a.Status)
	}
	if !a.LastSeen.Equal(t0) || a.LastSeen.Location() != time.UTC {
		t.Errorf("last_seen %v, want %v in UTC", a.LastSeen, t0)
	}

	update := seen("39c4a1", t0.Add(time.Minute), report("39c4a1", 48.81, 2.07, 3000))
	update.Status, update.Flight, update.OnGround = "signal_lost", "AFR108A", true
	s.Upsert(update)
	a, _ = s.GetByHex("39c4a1")
	if a.Status != "signal_lost" || a.Flight != "AFR108A" || !a.OnGround || !a.LastSeen.Equal(t0.Add(time.Minute)) {
		t.Errorf("the update should replace the state: %+v", a)
	}
	if n := s.Count(); n != 1 {
		t.Errorf("an update is not a new aircraft: %d rows", n)
	}
	if _, ok := s.GetByHex("absent"); ok {
		t.Error("an unknown hex should not be found")
	}
}

// The map trail is every position of the aircraft, oldest first; a report
// without a position is not drawn at 0,0.
func TestTheTrailIsChronologicalAndSkipsReportsWithoutPosition(t *testing.T) {
	s := newAircraftStore(t)
	s.Upsert(seen("39c4a1", t0.Add(20*time.Second), report("39c4a1", 48.83, 2.07, 3400)))
	s.Upsert(seen("39c4a1", t0, report("39c4a1", 48.81, 2.07, 3000)))
	noPos := report("39c4a1", 0, 0, 3100)
	noPos.Lat, noPos.Lon = nil, nil
	s.Upsert(seen("39c4a1", t0.Add(5*time.Second), noPos))
	s.Upsert(seen("39c4a1", t0.Add(10*time.Second), report("39c4a1", 48.82, 2.07, 3200)))

	a, ok := s.GetByHex("39c4a1")
	if !ok {
		t.Fatal("not found")
	}
	if len(a.History) != 3 {
		t.Fatalf("trail of %d points, want 3: %+v", len(a.History), a.History)
	}
	for i, want := range []float64{48.81, 48.82, 48.83} {
		if a.History[i].Lat != want {
			t.Errorf("point %d: lat %v, want %v", i, a.History[i].Lat, want)
		}
	}
	if !a.History[0].Timestamp.Equal(t0) || a.History[0].AltBaro != 3000 {
		t.Errorf("first point %+v", a.History[0])
	}
	// The latest message has a position, a speed and a track: a prediction is drawn.
	if a.ADSB == nil || len(a.Future) == 0 {
		t.Errorf("a moving aircraft with a position should have predicted positions, got %d", len(a.Future))
	}
}

// The aircraft card's history: the last hour, newest first, with a limit. These
// two queries compute "an hour ago" from the clock, so the rows are placed
// relative to it, half an hour clear of the boundary.
func TestRecentHistoryIsTheLastHourNewestFirst(t *testing.T) {
	s := newAircraftStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	for i, ago := range []time.Duration{2 * time.Hour, 30 * time.Minute, 20 * time.Minute, 10 * time.Minute} {
		s.Upsert(seen("39c4a1", now.Add(-ago), report("39c4a1", 48.80+float64(i)/100, 2.07, 3000)))
	}

	all, err := s.GetAllPositionHistory("39c4a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("%d positions in the last hour, want 3", len(all))
	}
	if !all[0].Timestamp.Equal(now.Add(-10*time.Minute)) || !all[2].Timestamp.Equal(now.Add(-30*time.Minute)) {
		t.Errorf("newest first: got %v .. %v", all[0].Timestamp, all[2].Timestamp)
	}
	for _, p := range all {
		if p.ID == nil || p.Lat == nil || p.SpeedGS == nil || *p.SpeedGS != 250 {
			t.Errorf("position read back incompletely: %+v", p)
		}
	}

	two, err := s.GetPositionHistoryWithLimit("39c4a1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(two) != 2 || !two[0].Timestamp.Equal(all[0].Timestamp) || !two[1].Timestamp.Equal(all[1].Timestamp) {
		t.Errorf("limit 2 should keep the two newest: %+v", two)
	}
	if two[0].Track == nil || *two[0].Track != 270 {
		t.Errorf("track should be read back: %+v", two[0])
	}
}

// "The latest position" is the newest by time, not the last row written: a
// late report must not become the aircraft's current position.
func TestTheLatestTargetIsTheNewestByTime(t *testing.T) {
	s := newAircraftStore(t)
	s.Upsert(seen("39c4a1", t0.Add(20*time.Second), report("39c4a1", 48.83, 2.07, 3400)))
	s.Upsert(seen("39c4a1", t0, report("39c4a1", 48.81, 2.07, 3000)))
	s.Upsert(seen("3c6444", t0, report("3c6444", 48.70, 2.30, 5000)))

	var newest int
	if err := s.GetDB().QueryRow(`SELECT id FROM adsb_targets WHERE aircraft_hex = '39c4a1' AND lat = 48.83`).Scan(&newest); err != nil {
		t.Fatal(err)
	}
	id, err := s.GetLatestADSBTargetID("39c4a1")
	if err != nil || id == nil || *id != newest {
		t.Errorf("latest id %v (%v), want %d", id, err, newest)
	}
	ids, err := s.GetLatestADSBTargetIDsBatch([]string{"39c4a1", "3c6444", "absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids["39c4a1"] == nil || *ids["39c4a1"] != newest || ids["3c6444"] == nil {
		t.Errorf("batch ids %v, want 39c4a1=%d and 3c6444", ids, newest)
	}
	if id, err := s.GetLatestADSBTargetID("absent"); err != nil || id != nil {
		t.Errorf("an unknown aircraft has no target: %v, %v", id, err)
	}
}

func phase(hex, p string, at time.Time, airport string) adsb.PhaseChangeInsert {
	return adsb.PhaseChangeInsert{Hex: hex, Flight: "AFR1081", Phase: p, Timestamp: at, Airport: airport}
}

// Phase changes are written in one batch and read back in every shape the map
// and the card use: full history, current phase, last take-off and landing.
func TestPhaseChangesAreReadBackInEveryShape(t *testing.T) {
	s := newAircraftStore(t)
	adsbID := 42
	to := phase("39c4a1", "T/O", t0.Add(5*time.Minute), "LFPG")
	to.ADSBId = &adsbID
	if err := s.InsertPhaseChangesBatch([]adsb.PhaseChangeInsert{
		phase("39c4a1", "TAX", t0, "LFPG"),
		to,
		phase("39c4a1", "CLB", t0.Add(6*time.Minute), ""),
		phase("39c4a1", "T/D", t0.Add(60*time.Minute), "LFPO"),
		phase("3c6444", "CRZ", t0.Add(time.Minute), ""),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertPhaseChangesBatch(nil); err != nil {
		t.Errorf("an empty batch is nothing to do: %v", err)
	}

	history, err := s.GetPhaseHistory("39c4a1")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ phase, airport string }{{"T/D", "LFPO"}, {"CLB", ""}, {"T/O", "LFPG"}, {"TAX", "LFPG"}}
	if len(history) != len(want) {
		t.Fatalf("history of %d, want %d: %+v", len(history), len(want), history)
	}
	for i, w := range want {
		if history[i].Phase != w.phase || history[i].Airport != w.airport {
			t.Errorf("history[%d] = %s/%q, want %s/%q", i, history[i].Phase, history[i].Airport, w.phase, w.airport)
		}
	}
	if history[2].ADSBId == nil || *history[2].ADSBId != 42 {
		t.Errorf("the take-off's adsb_id should be kept: %v", history[2].ADSBId)
	}

	current, err := s.GetCurrentPhase("39c4a1")
	if err != nil || current == nil || current.Phase != "T/D" {
		t.Errorf("current phase %+v (%v), want T/D", current, err)
	}
	if none, err := s.GetCurrentPhase("absent"); err != nil || none != nil {
		t.Errorf("no phase for an unknown aircraft: %+v, %v", none, err)
	}

	batch, err := s.GetCurrentPhasesBatch([]string{"39c4a1", "3c6444", "absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) != 2 || batch["39c4a1"].Phase != "T/D" || batch["3c6444"].Phase != "CRZ" {
		t.Errorf("current phases %+v", batch)
	}

	recent, err := s.getRecentPhaseHistoryBatch([]string{"39c4a1", "3c6444"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent["39c4a1"]) != 2 || recent["39c4a1"][0].Phase != "T/D" || recent["39c4a1"][1].Phase != "CLB" || len(recent["3c6444"]) != 1 {
		t.Errorf("the two most recent per aircraft: %+v", recent)
	}

	takeoff, err := s.GetLatestTakeoffTime("39c4a1")
	if err != nil || takeoff == nil || !takeoff.Equal(t0.Add(5*time.Minute)) {
		t.Errorf("take-off at %v (%v)", takeoff, err)
	}
	landing, err := s.GetLatestLandingTime("39c4a1")
	if err != nil || landing == nil || !landing.Equal(t0.Add(60*time.Minute)) {
		t.Errorf("landing at %v (%v)", landing, err)
	}
	if none, err := s.GetLatestLandingTime("3c6444"); err != nil || none != nil {
		t.Errorf("no landing for an aircraft in cruise: %v, %v", none, err)
	}
	takeoffs, err := s.GetLatestTakeoffTimesBatch([]string{"39c4a1", "3c6444"})
	if err != nil || len(takeoffs) != 1 || !takeoffs["39c4a1"].Equal(t0.Add(5*time.Minute)) {
		t.Errorf("take-offs %v (%v)", takeoffs, err)
	}
	landings, err := s.GetLatestLandingTimesBatch([]string{"39c4a1", "3c6444"})
	if err != nil || len(landings) != 1 || !landings["39c4a1"].Equal(t0.Add(60*time.Minute)) {
		t.Errorf("landings %v (%v)", landings, err)
	}
}

// The list the map polls: a filter on last_seen against the clock, with the
// current phase in both modes and the history and dates only in the full one.
func TestTheAircraftListFiltersOnLastSeenAndCarriesThePhase(t *testing.T) {
	s := newAircraftStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.Upsert(seen("39c4a1", now.Add(-time.Minute), report("39c4a1", 48.81, 2.07, 3000)))
	s.Upsert(seen("3c6444", now.Add(-30*time.Minute), report("3c6444", 48.70, 2.30, 5000)))
	if err := s.InsertPhaseChangesBatch([]adsb.PhaseChangeInsert{
		phase("39c4a1", "T/O", now.Add(-3*time.Minute), "LFPG"),
		phase("39c4a1", "CLB", now.Add(-2*time.Minute), "LFPG"),
	}); err != nil {
		t.Fatal(err)
	}

	recent := s.GetAllWithLastSeenFilter(5)
	if len(recent) != 1 || recent[0].Hex != "39c4a1" {
		t.Fatalf("seen in the last 5 minutes: %v", recent)
	}
	a := recent[0]
	if a.ADSB == nil || a.Phase == nil || a.Phase.Current[0].Phase != "CLB" || len(a.Phase.History) != 2 {
		t.Errorf("full mode should carry the message, the phase and its history: %+v", a.Phase)
	}
	if a.DateTookoff == nil || !a.DateTookoff.Equal(now.Add(-3*time.Minute)) {
		t.Errorf("take-off date %v", a.DateTookoff)
	}

	minimal := s.GetAllMinimal(5)
	if len(minimal) != 1 || minimal[0].Phase == nil || minimal[0].Phase.Current[0].Phase != "CLB" {
		t.Fatalf("minimal mode keeps the current phase: %+v", minimal)
	}
	if len(minimal[0].Phase.History) != 0 || minimal[0].DateTookoff != nil {
		t.Errorf("minimal mode skips history and dates: %+v, %v", minimal[0].Phase.History, minimal[0].DateTookoff)
	}

	if n := len(s.GetAll()); n != 2 {
		t.Errorf("no filter: %d aircraft, want 2", n)
	}
}

// The sweep that marks aircraft lost: active, not in the current report, and
// not heard since the cutoff.
func TestStaleAircraftAreActiveUnreportedAndOld(t *testing.T) {
	s := newAircraftStore(t)
	cutoff := t0.Add(-time.Minute)
	s.Upsert(seen("old", t0.Add(-5*time.Minute), report("old", 48.81, 2.07, 3000)))
	s.Upsert(seen("reported", t0.Add(-5*time.Minute), report("reported", 48.82, 2.07, 3000)))
	s.Upsert(seen("recent", t0, report("recent", 48.83, 2.07, 3000)))
	lost := seen("lost", t0.Add(-5*time.Minute), report("lost", 48.84, 2.07, 3000))
	lost.Status = "signal_lost"
	s.Upsert(lost)

	stale, err := s.GetStaleActiveAircraft([]string{"reported"}, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 || stale[0].Hex != "old" {
		t.Fatalf("stale %v, want only \"old\"", stale)
	}
	if stale[0].ADSB == nil || adsb.NumberOrZero(stale[0].ADSB.Lat) != 48.81 {
		t.Errorf("a stale aircraft keeps its last message, for landing detection: %+v", stale[0].ADSB)
	}
	all, err := s.GetStaleActiveAircraft(nil, cutoff)
	if err != nil || len(all) != 2 {
		t.Errorf("with no report, both old active aircraft are stale: %v (%v)", all, err)
	}
}

func TestGroundStateAndStatusFilter(t *testing.T) {
	s := newAircraftStore(t)
	ground := seen("ground", t0, report("ground", 48.72, 2.38, 0))
	ground.OnGround = true
	s.Upsert(ground)
	s.Upsert(seen("air", t0, report("air", 48.80, 2.10, 3000)))
	lost := seen("lost", t0, report("lost", 48.84, 2.07, 3000))
	lost.Status = "signal_lost"
	s.Upsert(lost)

	onGround, err := s.GetAircraftOnGroundBatch([]string{"ground", "air", "absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(onGround) != 2 || !onGround["ground"] || onGround["air"] {
		t.Errorf("on ground %v", onGround)
	}
	if _, known := onGround["absent"]; known {
		t.Error("a missing key means the aircraft is not known yet")
	}

	active := s.GetFiltered(0, 0, []string{"active"}, nil, nil, nil, nil)
	if len(active) != 2 {
		t.Errorf("active: %d aircraft, want 2", len(active))
	}
	for _, a := range active {
		if a.Status != "active" || a.ADSB == nil || a.Phase == nil {
			t.Errorf("filtered aircraft read back incompletely: %+v", a)
		}
	}
	if n := len(s.GetFiltered(0, 0, nil, nil, nil, nil, nil)); n != 3 {
		t.Errorf("no status filter: %d aircraft, want 3", n)
	}
}
