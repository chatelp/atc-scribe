package aip

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The real coding tables, as pdftotext -layout writes them, when they are at
// hand (AIP_CHART_TEXTS: a folder of AD_2_LFxx_DATA_(SID|STAR)…txt). Nothing
// of the AIP is in the repository.
//
// What it checks: the constraints read by column against a reading word by
// word, on the rows where every column holds a value or a dash (nine values
// after the point: fly-over, two directions, distance, turn, the two
// altitudes, speed, accuracy), where the word order alone is sure.
func TestRealTablesConstraintsAgreeWithTheWordOrder(t *testing.T) {
	dir := os.Getenv("AIP_CHART_TEXTS")
	if dir == "" {
		t.Skip("AIP_CHART_TEXTS not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "AD_2_LF*_DATA_S*.txt"))
	var note RoutesNote
	checked, agree, leftOut := 0, 0, 0
	var disagree []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(f)
		kind := "STAR"
		if strings.Contains(base, "_DATA_SID_") {
			kind = "SID"
		}
		routes := parseCodingTable(string(raw), base[5:9], kind, strings.TrimSuffix(base, ".txt"), &note)
		note.Routes += len(routes)
		// The word-order reading, for the rows it is sure of.
		byLine := map[string]Leg{}
		for _, r := range routes {
			for _, l := range r.Legs {
				byLine[itoa(l.line+1)] = l
			}
		}
		var cur string
		for ln, line := range strings.Split(string(raw), "\n") {
			if h := reProcHead.FindStringSubmatch(line); h != nil {
				cur = h[1]
				if !strings.Contains(cur, " ") {
					cur = cur[:len(cur)-2] + " " + cur[len(cur)-2:]
				}
				continue
			}
			loc := reLegLine.FindStringSubmatchIndex(line)
			if loc == nil || line[loc[4]:loc[5]] == "-" {
				continue
			}
			var after []string
			for _, tk := range tokens(line[loc[5]:]) {
				after = append(after, tk.text)
			}
			// Nine values, the last an accuracy in NM: a table whose last
			// column is a specification ("RNAV 1") has one word too many.
			if len(after) != 9 || !reAccuracy.MatchString(after[8]) {
				continue
			}
			l, ok := byLine[itoa(ln+1)]
			if !ok {
				continue
			}
			norm := func(v string) string {
				if v == "-" {
					return ""
				}
				if strings.HasPrefix(v, "FL") {
					return "FL" + strings.TrimPrefix(strings.TrimPrefix(v, "FL"), " ")
				}
				return v
			}
			if !l.read {
				leftOut++
				continue
			}
			kt := ""
			if l.MaxKt > 0 {
				kt = itoa(l.MaxKt)
			}
			checked++
			if l.MinAlt == norm(after[5]) && l.MaxAlt == norm(after[6]) && kt == norm(after[7]) {
				agree++
			} else if len(disagree) < 10 {
				disagree = append(disagree, base+": "+strings.TrimSpace(line)+" -> min "+l.MinAlt+" max "+l.MaxAlt+" kt "+kt)
			}
		}
	}
	t.Logf("%s", routesNoteText(note))
	t.Logf("constraints checked against the word order: %d rows, %d agree; %d sure rows left out by the column reading", checked, agree, leftOut)
	for _, d := range disagree {
		t.Logf("  differs: %s", d)
	}
	if checked == 0 || agree < checked*99/100 {
		t.Errorf("the column reading agrees on %d of %d rows", agree, checked)
	}
}

var reAccuracy = regexp.MustCompile(`^\d[.,]\d+$`)

func itoa(n int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
