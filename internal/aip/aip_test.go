package aip

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestTheCycleInForceStepsEvery28Days(t *testing.T) {
	for _, c := range []struct{ on, want string }{
		{"2026-10-01", "2026-10-01"},
		{"2026-10-15", "2026-10-01"},
		{"2026-10-28", "2026-10-01"},
		{"2026-10-29", "2026-10-29"},
		{"2026-09-30", "2026-09-03"},
		{"2027-01-21", "2027-01-21"},
	} {
		if got := InForce(day(c.on)).Format("2006-01-02"); got != c.want {
			t.Errorf("on %s: %s, want %s", c.on, got, c.want)
		}
	}
}

func TestTheSIAPublishesACycleUnderItsDate(t *testing.T) {
	want := "https://www.sia.aviation-civile.gouv.fr/media/dvd/eAIP_01_OCT_2026/FRANCE/AIRAC-2026-10-01/html/eAIP"
	if got := BaseURL(day("2026-10-01")); got != want {
		t.Fatalf("%s\nwant %s", got, want)
	}
}

// Extracts written like the eAIP's pages: a table row per point, call signs
// and procedures in the text.
const enr44 = `<table><tr><td>ELCOB</td><td>49°10'00.0"N 001°50'00.0"E</td></tr>
<tr><td>FARAW</td><td>43°00'00.0"N 005°00'00.0"E</td></tr>
<tr><td>Name</td><td>coordinates</td></tr></table>`

const ad2 = `<html><script>var x = "OKIPA 9Z";</script><p>CHARLES DE GAULLE Approche 119.250 MHZ.
NIL Départ. Departures OKIPA 1W and MOPAR 1E.</p></html>`

func TestPointsWithinTheRadiusCallSignsAndProcedures(t *testing.T) {
	n := Parse(map[string]string{pagePoints: enr44, "FR-AD-2.LFPG-fr-FR.html": ad2}, 49.0, 2.5, 120)
	if len(n.Points) != 1 || n.Points[0].Name != "ELCOB" {
		t.Fatalf("points %+v, want ELCOB alone (FARAW is far)", n.Points)
	}
	if n.Points[0].Lat < 49.16 || n.Points[0].Lat > 49.17 || n.Points[0].NM < 25 || n.Points[0].NM > 35 {
		t.Errorf("ELCOB at %+v", n.Points[0])
	}
	if strings.Join(n.Callsigns, "|") != "CHARLES DE GAULLE Approche" {
		t.Errorf("call signs %q: NIL is a table word, not a name", n.Callsigns)
	}
	if strings.Join(n.Procedures, "|") != "MOPAR 1E|OKIPA 1W" {
		t.Errorf("procedures %q: a script is not text", n.Procedures)
	}
}

// A point moved this cycle: ENR 4.4 prints the old coordinates struck out
// beside the new ones, as on 01 OCT 2026 for DOPAP.
func TestAPointMovedThisCycleTakesItsNewCoordinates(t *testing.T) {
	page := `<table><tr id="SP-DOPAP"><td><span>DOPAP</span></td><td><p class="line"><span>
<del class="cellChanged AmdtDeletedAIRAC" title="x">48°42'07.8"N</del>
<ins class="cellChanged AmdtInsertedAIRAC" title="x">48°42'18.0"N</ins></span></p><p class="line"><span>
<del class="cellChanged AmdtDeletedAIRAC">001°58'00.0"E</del>
<ins class="cellChanged AmdtInsertedAIRAC">001°59'00.0"E</ins></span></p></td></tr></table>`
	n := Parse(map[string]string{pagePoints: page}, 49.0, 2.5, 120)
	if len(n.Points) != 1 {
		t.Fatalf("points %+v, want DOPAP", n.Points)
	}
	p := n.Points[0]
	if math.Abs(p.Lat-(48+42.0/60+18.0/3600)) > 1e-4 || math.Abs(p.Lon-(1+59.0/60)) > 1e-4 {
		t.Errorf("DOPAP at %v %v, want the inserted coordinates", p.Lat, p.Lon)
	}
}

func TestMakeFetchesOnceAndWritesTheListWithItsSource(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch {
		case strings.HasSuffix(r.URL.Path, pagePoints):
			w.Write([]byte(enr44))
		default:
			w.Write([]byte(ad2))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	m := &Maker{OutPath: filepath.Join(dir, "aip-names.json"), CacheDir: filepath.Join(dir, "cache"),
		Client: &http.Client{Transport: rewrite{srv.URL}}, Now: func() time.Time { return day("2026-10-15") }}
	req := Request{Lat: 49, Lon: 2.5, RadiusNM: 120, Airports: []string{"lfpg"}}
	n, err := m.Make(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if n.AIRAC != "2026-10-01" || n.NextAIRAC != "2026-10-29" || !strings.Contains(n.Source, "SIA (DGAC)") ||
		!strings.Contains(n.Source, "ENR 4.4, ENR 2.1, AD 2 LFPG") {
		t.Errorf("list %+v", n)
	}
	if hits.Load() != 3 {
		t.Errorf("%d requests, want one per page", hits.Load())
	}
	back, err := Load(m.OutPath)
	if err != nil || len(back.Points) != 1 {
		t.Fatalf("read back %+v, %v", back, err)
	}
	// The pages are kept for the cycle: a second list asks nothing.
	if _, err := m.Make(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 3 {
		t.Errorf("%d requests after a second list, want none more", hits.Load())
	}
	if st := m.State(); st.Refreshing || st.LastError != "" {
		t.Errorf("state %+v", st)
	}
}

func TestAPageTheSIARefusesLeavesNoListAndSaysWhy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	dir := t.TempDir()
	m := &Maker{OutPath: filepath.Join(dir, "aip-names.json"), CacheDir: filepath.Join(dir, "cache"),
		Client: &http.Client{Transport: rewrite{srv.URL}}}
	if _, err := m.Make(context.Background(), Request{Lat: 49, Lon: 2.5, RadiusNM: 120}); err == nil {
		t.Fatal("no error for a 404")
	}
	if _, err := os.Stat(m.OutPath); !os.IsNotExist(err) {
		t.Error("a list was written")
	}
	if st := m.State(); !strings.Contains(st.LastError, "404") {
		t.Errorf("last error %q", st.LastError)
	}
}

// rewrite sends every request to the test server, keeping its path.
type rewrite struct{ base string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	target, _ := http.NewRequest(req.Method, r.base+u.Path, nil)
	return http.DefaultTransport.RoundTrip(target)
}

func TestAnAirportWithoutAnAD2PageIsSkippedAndNamed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, pagePoints):
			w.Write([]byte(enr44))
		case strings.Contains(r.URL.Path, "LFPZ"):
			http.NotFound(w, r)
		default:
			w.Write([]byte(ad2))
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	m := &Maker{OutPath: filepath.Join(dir, "aip-names.json"), CacheDir: filepath.Join(dir, "cache"),
		Client: &http.Client{Transport: rewrite{srv.URL}}}
	n, err := m.Make(context.Background(), Request{Lat: 49, Lon: 2.5, RadiusNM: 120, Airports: []string{"LFPG", "LFPZ"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Skipped) != 1 || !strings.HasPrefix(n.Skipped[0], "LFPZ") || strings.Contains(n.Source, "LFPZ") {
		t.Errorf("skipped %q, source %q", n.Skipped, n.Source)
	}
}

func TestOnlyFrenchAirportsAreRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, pagePoints) {
			w.Write([]byte(enr44))
			return
		}
		w.Write([]byte(ad2))
	}))
	defer srv.Close()
	dir := t.TempDir()
	m := &Maker{OutPath: filepath.Join(dir, "aip-names.json"), CacheDir: filepath.Join(dir, "cache"),
		Client: &http.Client{Transport: rewrite{srv.URL}}}
	if _, err := m.Make(context.Background(), Request{Lat: 43.7, Lon: -79.6, RadiusNM: 120, Airports: []string{"CYYZ"}}); err == nil ||
		!strings.Contains(err.Error(), "CYYZ") {
		t.Errorf("a station following CYYZ alone: %v", err)
	}
	n, err := m.Make(context.Background(), Request{Lat: 49, Lon: 2.5, RadiusNM: 120, Airports: []string{"LFPG", "EGLL"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(n.Outside, ",") != "EGLL" || strings.Join(n.Airports, ",") != "LFPG" {
		t.Errorf("outside %v, airports %v", n.Outside, n.Airports)
	}
	// The text names OKIPA 1W and MOPAR 1E; their points are not in the test
	// table of points, so they have no place to show.
	for _, p := range n.ProcedurePoints {
		t.Errorf("a procedure point without coordinates: %+v", p)
	}
}
