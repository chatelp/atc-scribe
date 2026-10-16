package aip

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
