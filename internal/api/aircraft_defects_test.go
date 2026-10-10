package api

import "testing"

// The altitude filters are parsed, passed down, and dropped on the floor:
// sqlite.AircraftStorage.GetFiltered filters by status only. Nothing in www/
// sends them, so the page does not meet it.
func TestTheAltitudeFiltersSelectByAltitude(t *testing.T) {
	skipDefect(t, "min_altitude and max_altitude are accepted by /aircraft and ignored "+
		"(internal/storage/sqlite/aircraft.go, GetFiltered: status only)")
	s := aircraftStack(t)
	var page aircraftPage
	s.get("/api/v1/aircraft?min_altitude=10000", &page)
	if page.Count != 2 || page.byHex(hexCruise) == nil || page.byHex(hexClimb) == nil {
		t.Errorf("min_altitude=10000: %v, want the two above 10 000 ft", page.hexes())
	}
	s.get("/api/v1/aircraft?max_altitude=5000", &page)
	if page.Count != 2 || page.byHex(hexDescent) == nil || page.byHex(hexGround) == nil {
		t.Errorf("max_altitude=5000: %v, want the two below 5 000 ft", page.hexes())
	}
}
