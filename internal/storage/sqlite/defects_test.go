package sqlite

import (
	"os"
	"testing"
	"time"
)

// skipDefect marks a test that shows a defect found while writing these tests.
// It is skipped so the suite stays green until the fix is made on the station's
// side; ATC_DEFECTS=1 runs it, to see the failure.
func skipDefect(t *testing.T, what string) {
	t.Helper()
	if os.Getenv("ATC_DEFECTS") == "" {
		t.Skip("defect: " + what)
	}
}

// SQLite treats NULLs as distinct in a UNIQUE constraint, so a report with any
// of lat, lon, alt_baro, gs, tas or track missing is never recognised as a
// repeat. tas, for one, is only there when the receiver decoded it from a
// Comm-B reply, so many reports go without it.
func TestARepeatedPositionWithoutAirspeedIsStoredOnce(t *testing.T) {
	s := newAircraftStore(t)
	for i := 0; i < 3; i++ {
		r := report("3949e2", 48.77, 2.10, 0)
		r.TAS = nil
		s.Upsert(seen("3949e2", t0.Add(time.Duration(i)*time.Second), r))
	}
	if n := positionsOf(t, s, "3949e2"); n != 1 {
		t.Errorf("the same report without tas three times: %d rows, want 1", n)
	}
}

// Not never again, though: an aircraft without a position holding its level,
// track and speed must still be found around a transmission minutes later. An
// unchanged state is written again after 10 s.
func TestAnUnchangedStateIsWrittenAgainEveryTenSeconds(t *testing.T) {
	s := newAircraftStore(t)
	for _, sec := range []int{0, 1, 2, 9, 11, 12, 25} {
		r := report("3949e2", 48.77, 2.10, 0)
		r.TAS = nil
		s.Upsert(seen("3949e2", t0.Add(time.Duration(sec)*time.Second), r))
	}
	// kept: 0 s, 11 s (more than 10 s after 0), 25 s (more than 10 s after 11)
	if n := positionsOf(t, s, "3949e2"); n != 3 {
		t.Errorf("an unchanged state over 25 s: %d rows, want 3 (0, 11 and 25 s)", n)
	}
}

// created_at is written as CreatedAt.Format(time.RFC3339), keeping the zone it
// was given in, and every query compares or sorts it as text. The local
// transcription dates a transmission from time.Now(), in the host's zone, so
// on a host that is not on UTC the stored text is local time with its offset:
// a range given in UTC misses it, and on the night the clocks go back the
// hour that repeats sorts out of order.
func TestTranscriptionTimesAreComparedWhateverTheirZone(t *testing.T) {
	s := newTranscriptionStore(t)
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no time zone database:", err)
	}

	noon := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	id := store(t, s, heard("125825", noon.In(paris), "dated 14:00+02:00"))
	got, err := s.GetTranscriptionsByTimeRange(noon.Add(-30*time.Minute), noon.Add(30*time.Minute), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != id {
		t.Errorf("11:30Z to 12:30Z should find the transmission of 12:00Z, got %v", ids(got))
	}

	// 25 October 2026: 03:00 CEST becomes 02:00 CET. 02:50+02:00 is 00:50Z,
	// 02:10+01:00 is 01:10Z, twenty minutes later.
	before := store(t, s, heard("120850", time.Date(2026, 10, 25, 0, 50, 0, 0, time.UTC).In(paris), "before"))
	after := store(t, s, heard("120850", time.Date(2026, 10, 25, 1, 10, 0, 0, time.UTC).In(paris), "after"))
	newest, err := s.GetTranscriptionsByFrequency("120850", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(newest); len(got) != 2 || got[0] != after || got[1] != before {
		t.Errorf("newest first across the change of time: %v, want [%d %d]", got, after, before)
	}
}
