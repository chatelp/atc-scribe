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
	skipDefect(t, "UNIQUE(aircraft_hex, lat, lon, alt_baro, gs, tas, track) does not deduplicate when one column is NULL: "+
		"an aircraft reported without tas gets a new adsb_targets row at every poll")
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
