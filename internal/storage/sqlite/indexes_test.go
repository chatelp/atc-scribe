package sqlite

import (
	"testing"
	"time"
)

func indexesOf(t *testing.T, s *AircraftStorage, table string) map[string]bool {
	t.Helper()
	rows, err := s.GetDB().Query(`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ?`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names[name] = true
	}
	return names
}

// Two upstream indexes on adsb_targets duplicate others and no query uses them:
// a new file does not get them, and a file that had them loses them on open.
// The index behind the UNIQUE constraint, which INSERT OR IGNORE relies on to
// skip a position already stored, stays.
func TestRedundantPositionIndexesAreGone(t *testing.T) {
	log := testLog(t)
	path := DailyPath(t.TempDir(), time.Now())
	s, err := NewAircraftStorage(path, log)
	if err != nil {
		t.Fatal(err)
	}
	redundant := []string{"idx_adsb_targets_aircraft_hex", "idx_adsb_targets_unique_check"}
	idx := indexesOf(t, s, "adsb_targets")
	for _, name := range redundant {
		if idx[name] {
			t.Errorf("a new file should not have %s", name)
		}
	}
	if !idx["sqlite_autoindex_adsb_targets_1"] || !idx["idx_adsb_targets_hex_timestamp"] {
		t.Errorf("the UNIQUE constraint's index and hex_timestamp must stay: %v", idx)
	}

	// A file written by an earlier version still has them.
	for _, stmt := range []string{
		`CREATE INDEX idx_adsb_targets_aircraft_hex ON adsb_targets(aircraft_hex)`,
		`CREATE INDEX idx_adsb_targets_unique_check ON adsb_targets(aircraft_hex, lat, lon, alt_baro, gs, tas, track)`,
	} {
		if _, err := s.GetDB().Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s, err = NewAircraftStorage(path, log)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	idx = indexesOf(t, s, "adsb_targets")
	for _, name := range redundant {
		if idx[name] {
			t.Errorf("%s should be dropped when an older file is opened", name)
		}
	}
}
