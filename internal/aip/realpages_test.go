package aip

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// With AIP_PAGES set to a folder of saved eAIP pages (ENR 4.4, ENR 2.1, AD 2
// LFPG, LFPO, LFPB) and AIP_REFERENCE to a list made from them by another
// tool, the two must agree. Skipped otherwise: no AIP data in the repository.
func TestRealPagesAgreeWithAReferenceList(t *testing.T) {
	dir, ref := os.Getenv("AIP_PAGES"), os.Getenv("AIP_REFERENCE")
	if dir == "" || ref == "" {
		t.Skip("AIP_PAGES and AIP_REFERENCE not set")
	}
	pages := map[string]string{}
	for _, name := range PageNames([]string{"LFPG", "LFPO", "LFPB"}) {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		pages[name] = string(raw)
	}
	n := Parse(pages, 49.0097, 2.5479, 120)
	var want struct {
		Points     []struct{ Nom string } `json:"points"`
		Callsigns  []string               `json:"indicatifs_atc"`
		Procedures []string               `json:"procedures"`
	}
	raw, err := os.ReadFile(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range n.Points {
		got[p.Name] = true
	}
	var missing []string
	for _, p := range want.Points {
		if !got[p.Nom] {
			missing = append(missing, p.Nom)
		}
	}
	sort.Strings(want.Callsigns)
	t.Logf("points %d (reference %d), missing %v; call signs %d (reference %d); procedures %v", len(n.Points), len(want.Points), missing, len(n.Callsigns), len(want.Callsigns), n.Procedures)
	// This parser finds more call signs than the reference (PARIS Contrôle,
	// REIMS Contrôle: real positions of ENR 2.1); every one of the reference's
	// must be among them.
	ours := map[string]bool{}
	for _, c := range n.Callsigns {
		ours[c] = true
	}
	for _, c := range want.Callsigns {
		if !ours[c] {
			t.Errorf("call sign %q of the reference missing", c)
		}
	}
	if strings.Join(n.Procedures, "|") != strings.Join(want.Procedures, "|") {
		t.Errorf("procedures differ")
	}
	if len(missing) > 1 {
		t.Errorf("points missing: %v", missing)
	}
}

// With AIP_PAGES and AIP_CHARTS (a folder of the SID and STAR coding tables,
// as Cartes/LFPG/... beside the pages), the procedure points of Paris come out
// without a request: OPALE, ELCOB, ATREX, RANUX, NURMO, and MOPAR, OKIPA from
// the text. Needs pdftotext.
func TestRealChartsGiveTheProcedurePoints(t *testing.T) {
	pagesDir, charts := os.Getenv("AIP_PAGES"), os.Getenv("AIP_CHARTS")
	if pagesDir == "" || charts == "" || pdfToText() == "" {
		t.Skip("AIP_PAGES, AIP_CHARTS or pdftotext missing")
	}
	cache := t.TempDir()
	folder := filepath.Join(cache, "2026-10-01")
	if err := os.MkdirAll(filepath.Join(folder, "Cartes", "LFPG"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(folder, "Cartes", "LFPO"), 0o755)
	for _, name := range PageNames([]string{"LFPG", "LFPO"}) {
		b, err := os.ReadFile(filepath.Join(pagesDir, name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(folder, name), b, 0o644)
	}
	pdfs, _ := filepath.Glob(filepath.Join(charts, "AD_2_*.pdf"))
	for _, p := range pdfs {
		b, _ := os.ReadFile(p)
		ad := filepath.Base(p)[5:9]
		os.WriteFile(filepath.Join(folder, "Cartes", ad, filepath.Base(p)), b, 0o644)
	}
	m := &Maker{OutPath: filepath.Join(cache, "aip-names.json"), CacheDir: cache,
		Client: &http.Client{Transport: refuse{}}, Now: func() time.Time { return time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC) }}
	n, err := m.Make(context.Background(), Request{Lat: 49.0097, Lon: 2.5479, RadiusNM: 120, Airports: []string{"LFPG", "LFPO"}})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, p := range n.ProcedurePoints {
		names[p.Name] = true
	}
	t.Logf("%d charts read, %d procedure points: %v", n.ChartsRead, len(n.ProcedurePoints), n.ProcedurePoints)
	for _, want := range []string{"OPALE", "ELCOB", "ATREX", "RANUX", "NURMO", "MOPAR", "OKIPA"} {
		if !names[want] {
			t.Errorf("%s missing", want)
		}
	}
}

// refuse answers every request with an error: nothing may be downloaded.
type refuse struct{}

func (refuse) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("no network in this test")
}
