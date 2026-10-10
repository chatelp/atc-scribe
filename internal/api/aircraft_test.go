package api

import (
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/storage/sqlite"
)

// What /aircraft answers, read with the field names the page reads.
type aircraftPage struct {
	Count    int                 `json:"count"`
	Counts   adsb.AircraftCounts `json:"counts"`
	Aircraft []*adsb.Aircraft    `json:"aircraft"`
}

func (p aircraftPage) byHex(hex string) *adsb.Aircraft {
	for _, a := range p.Aircraft {
		if a.Hex == hex {
			return a
		}
	}
	return nil
}

func (p aircraftPage) hexes() []string {
	out := make([]string, 0, len(p.Aircraft))
	for _, a := range p.Aircraft {
		out = append(out, a.Hex)
	}
	return out
}

func near(t *testing.T, what string, got *float64, want, tolerance float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: missing, want about %.1f", what, want)
		return
	}
	if math.Abs(*got-want) > tolerance {
		t.Errorf("%s = %.1f, want %.1f ± %.1f", what, *got, want, tolerance)
	}
}

// A signed-in stack whose cruise aircraft has been heard on the radio, cleared
// to land, and judged in cruise: everything /aircraft hangs on a target.
func aircraftStack(t *testing.T) *stack {
	t.Helper()
	s := newStack(t, stackOptions{})
	s.signIn()
	at := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	id := s.transcribe("mix", "air france one two three descend flight level one one zero", "AFR123", at)
	if _, err := s.clr.StoreClearance(&sqlite.ClearanceRecord{TranscriptionID: id, Callsign: "AFR123",
		ClearanceType: "landing", ClearanceText: "cleared to land runway 27L", Runway: "27L",
		Timestamp: at, Status: "issued"}); err != nil {
		t.Fatal(err)
	}
	for i, phase := range []string{"CLB", "CRZ"} {
		if err := s.aircraft.InsertPhaseChange(hexCruise, "AFR123", phase, at.Add(time.Duration(i)*30*time.Minute), nil); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestTheReceiversAircraftComeWithDistanceVoiceClearancesAndPhase(t *testing.T) {
	s := aircraftStack(t)
	var page aircraftPage
	s.get("/api/v1/aircraft", &page)
	if page.Count != aircraftSeen || len(page.Aircraft) != aircraftSeen {
		t.Fatalf("count %d with %d aircraft, want %d: %v", page.Count, len(page.Aircraft), aircraftSeen, page.hexes())
	}
	if want := (adsb.AircraftCounts{GroundActive: 1, GroundTotal: 1, AirActive: 3, AirTotal: 3}); page.Counts != want {
		t.Errorf("counts = %+v, want %+v", page.Counts, want)
	}

	afr := page.byHex(hexCruise)
	if afr == nil {
		t.Fatal("the cruise aircraft is missing")
	}
	if afr.Flight != "AFR123" || afr.Status != "active" || afr.OnGround {
		t.Errorf("AFR123 = flight %q status %q on_ground %v", afr.Flight, afr.Status, afr.OnGround)
	}
	near(t, "AFR123 distance from the station", afr.Distance, 25.7, 0.5)
	if afr.ADSB == nil || afr.ADSB.AltBaro.Float64() != 35000 || afr.ADSB.Squawk != "1000" {
		t.Errorf("AFR123 ADS-B block = %+v, want FL350 squawk 1000", afr.ADSB)
	}
	if afr.Voice == nil || afr.Voice.Transmissions != 1 || afr.Voice.LastText == "" {
		t.Errorf("AFR123 voice = %+v, want one transmission with its text", afr.Voice)
	}
	if len(afr.Clearances) != 1 || afr.Clearances[0].Type != "landing" || afr.Clearances[0].Runway != "27L" ||
		afr.Clearances[0].TimeSinceIssued == "" {
		t.Errorf("AFR123 clearances = %+v, want the landing clearance with its age", afr.Clearances)
	}
	if afr.Phase == nil || len(afr.Phase.Current) != 1 || afr.Phase.Current[0].Phase != "CRZ" || len(afr.Phase.History) != 2 {
		t.Errorf("AFR123 phase = %+v, want CRZ current with two in the history", afr.Phase)
	}

	ground := page.byHex(hexGround)
	if ground == nil || !ground.OnGround || ground.Voice != nil {
		t.Errorf("the ground aircraft = %+v, want on the ground and silent", ground)
	}
	for _, a := range page.Aircraft {
		if a.Distance == nil {
			t.Errorf("%s has no distance", a.Hex)
		}
	}
}

func TestAircraftFilters(t *testing.T) {
	s := aircraftStack(t)
	var page aircraftPage

	s.get("/api/v1/aircraft?callsign=afr", &page)
	if page.Count != 1 || page.Aircraft[0].Hex != hexCruise {
		t.Errorf("callsign=afr: %v, want the cruise aircraft only (case-insensitive, substring)", page.hexes())
	}
	s.get("/api/v1/aircraft?status=active", &page)
	if page.Count != aircraftSeen {
		t.Errorf("status=active: %d, want %d", page.Count, aircraftSeen)
	}
	s.get("/api/v1/aircraft?status=signal_lost", &page)
	if page.Count != 0 {
		t.Errorf("status=signal_lost: %v, want none, every aircraft was just seen", page.hexes())
	}
	s.get("/api/v1/aircraft?status=signal_lost,active", &page)
	if page.Count != aircraftSeen {
		t.Errorf("status=signal_lost,active: %d, want %d", page.Count, aircraftSeen)
	}
	s.get("/api/v1/aircraft?last_seen_minutes=10", &page)
	if page.Count != aircraftSeen {
		t.Errorf("last_seen_minutes=10: %d, want %d", page.Count, aircraftSeen)
	}
	// The ground aircraft is 13 NM from the station: parked at another airport.
	s.get("/api/v1/aircraft?exclude_other_airports_grounded=1", &page)
	if page.Count != aircraftSeen-1 || page.byHex(hexGround) != nil {
		t.Errorf("exclude_other_airports_grounded=1: %v, want the ground aircraft gone", page.hexes())
	}
	// Unparseable values fall back to the defaults rather than failing.
	s.get("/api/v1/aircraft?last_seen_minutes=soon&status=", &page)
	if page.Count != aircraftSeen {
		t.Errorf("unparseable filters: %d, want %d", page.Count, aircraftSeen)
	}
}

// A proximity query: the aircraft around a reference one, nearest first,
// with the distance and bearing from it, and without the reference itself
// or anything on the ground.
func TestAProximityQueryIsSortedByDistanceFromTheReference(t *testing.T) {
	s := aircraftStack(t)
	var page aircraftPage
	s.get(fmt.Sprintf("/api/v1/aircraft?distance_nm=30&ref_hex=%s", hexCruise), &page)
	if got, want := page.hexes(), []string{hexDescent, hexClimb}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("within 30 NM of AFR123: %v, want %v (nearest first, no reference, no ground)", got, want)
	}
	near(t, "RYR45 relative distance", page.Aircraft[0].RelativeDistance, 16.9, 0.5)
	near(t, "EZY77 relative distance", page.Aircraft[1].RelativeDistance, 18.4, 0.5)
	near(t, "RYR45 distance from the station", page.Aircraft[0].Distance, 18.2, 0.5)
	if page.Aircraft[0].RelativeBearing == nil || page.Aircraft[0].RelativeAlt == nil {
		t.Error("a reference with a heading gives each aircraft a relative bearing and altitude")
	}
	near(t, "RYR45 relative altitude", page.Aircraft[0].RelativeAlt, -32000, 1)
	if page.Aircraft[0].History != nil {
		t.Error("a proximity query carries no history, to keep the payload small")
	}
	s.get(fmt.Sprintf("/api/v1/aircraft?distance_nm=17.5&ref_hex=%s", hexCruise), &page)
	if got := page.hexes(); fmt.Sprint(got) != fmt.Sprint([]string{hexDescent}) {
		t.Errorf("within 17.5 NM: %v, want RYR45 alone", got)
	}
	// The same from coordinates, and from a flight number.
	s.get("/api/v1/aircraft?distance_nm=30&ref_lat=48.80&ref_lon=2.70", &page)
	if got, want := page.hexes(), []string{hexCruise, hexDescent, hexClimb}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("within 30 NM of a point: %v, want %v", got, want)
	}
	s.get("/api/v1/aircraft?distance_nm=30&ref_flight=afr123", &page)
	if got, want := page.hexes(), []string{hexCruise, hexDescent, hexClimb}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("within 30 NM of a flight number: %v, want %v", got, want)
	}
}

func TestTheSimpleListingIsLight(t *testing.T) {
	s := aircraftStack(t)
	var page struct {
		Count    int                   `json:"count"`
		Aircraft []adsb.AircraftSimple `json:"aircraft"`
		Counts   *adsb.AircraftCounts  `json:"counts"`
	}
	s.get("/api/v1/aircraft?simple=1", &page)
	if page.Count != aircraftSeen || len(page.Aircraft) != aircraftSeen || page.Counts != nil {
		t.Fatalf("simple=1: count %d, %d aircraft, counts %v", page.Count, len(page.Aircraft), page.Counts)
	}
	var afr *adsb.AircraftSimple
	for i := range page.Aircraft {
		if page.Aircraft[i].Hex == hexCruise {
			afr = &page.Aircraft[i]
		}
	}
	if afr == nil {
		t.Fatal("the cruise aircraft is missing")
	}
	if afr.Callsign != "AFR123" || afr.AltBaro != 35000 || afr.Squawk != "1000" || afr.Phase != "CRZ" ||
		afr.Status != "active" || afr.GroundSpeed == nil || *afr.GroundSpeed != 450 {
		t.Errorf("simple AFR123 = %+v", *afr)
	}
	near(t, "simple AFR123 distance", afr.Distance, 25.7, 0.5)
	// A light answer carries no ADS-B block, no history, no clearances.
	var raw struct {
		Aircraft []map[string]any `json:"aircraft"`
	}
	s.get("/api/v1/aircraft?simple=1", &raw)
	for _, a := range raw.Aircraft {
		for _, heavy := range []string{"adsb", "history", "clearances", "voice", "future"} {
			if _, ok := a[heavy]; ok {
				t.Errorf("simple=1 carries %q for %v", heavy, a["hex"])
			}
		}
	}
}

func TestOneAircraftByHex(t *testing.T) {
	s := aircraftStack(t)
	var a adsb.Aircraft
	s.get("/api/v1/aircraft/"+hexCruise, &a)
	if a.Hex != hexCruise || a.Flight != "AFR123" || a.ADSB == nil || a.ADSB.AltBaro.Float64() != 35000 {
		t.Errorf("by hex: %+v", a)
	}
	near(t, "distance", a.Distance, 25.7, 0.5)
	if a.Phase == nil || len(a.Phase.Current) != 1 || a.Phase.Current[0].Phase != "CRZ" {
		t.Errorf("phase = %+v, want CRZ", a.Phase)
	}
	if rec := s.do("GET", "/api/v1/aircraft/ffffff", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown hex: %d, want 404", rec.Code)
	}
}

// The tracks the map draws: the positions stored (one per fetch here), each
// with its distance; the predicted ones; the phases, newest first.
func TestTracksCarryHistoryPredictionAndPhases(t *testing.T) {
	s := aircraftStack(t)
	var tr adsb.AircraftTracksResponse
	s.get("/api/v1/aircraft/"+hexCruise+"/tracks", &tr)
	if tr.Hex != hexCruise || tr.Flight != "AFR123" {
		t.Errorf("tracks for %s %s", tr.Hex, tr.Flight)
	}
	near(t, "current distance", tr.Distance, 25.7, 0.5)
	if len(tr.History) != 1 {
		t.Fatalf("history has %d positions, want the one fetch", len(tr.History))
	}
	h := tr.History[0]
	if h.Lat == nil || h.Lon == nil || *h.Lat != 48.8 || *h.Lon != 2.7 || h.Altitude == nil || *h.Altitude != 35000 {
		t.Errorf("history[0] = %+v", h)
	}
	near(t, "history[0] distance", h.Distance, 25.7, 0.5)
	if len(tr.Future) == 0 {
		t.Error("an aircraft with a track and a speed gets predicted positions")
	}
	for i, p := range tr.Future {
		if p.Distance == nil {
			t.Errorf("future[%d] has no distance", i)
			break
		}
	}
	if len(tr.PhaseHistory) != 2 || tr.PhaseHistory[0].Phase != "CRZ" || tr.PhaseHistory[1].Phase != "CLB" {
		t.Errorf("phase history = %+v, want CRZ then CLB (newest first)", tr.PhaseHistory)
	}
	if rec := s.do("GET", "/api/v1/aircraft/ffffff/tracks", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown hex: %d, want 404", rec.Code)
	}
	// The ground aircraft has a position but neither speed nor altitude to
	// predict from.
	s.get("/api/v1/aircraft/"+hexGround+"/tracks?limit=5", &tr)
	if len(tr.History) != 1 || len(tr.Future) != 0 {
		t.Errorf("ground aircraft: %d positions, %d predicted, want 1 and 0", len(tr.History), len(tr.Future))
	}
}
