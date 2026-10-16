package aip

// The published routes of the followed airports' departures and arrivals,
// drawn on the map (Q56): from the SID and STAR coding tables of the eAIP,
// each procedure's points in order, its runways, and per segment the
// altitude and speed constraints the table gives.
//
// Where the coordinates come from: the named points from ENR 4.4 (all of
// France, not the station's radius), the navaids from ENR 4.1, the
// technical points of an airport (PG271…) from its DATA sheets (AD 2 LFPG
// DATA 01…), the runway thresholds from its AD 2.12. A route whose points
// are not all placed is kept, marked incomplete: the page draws a route
// whole or not at all (design, ProcedureRoute).
//
// The constraints are read by column: a table's header names its columns
// ("MNM Altitude", "MAX IAS (kt)"…) and a value belongs to the column it
// sits under. A value whose column is not clear is left out, never guessed:
// a missing constraint misleads less than a wrong one.

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Route is one published procedure.
type Route struct {
	Airport string `json:"airport"`
	Name    string `json:"name"` // "ELCOB 7G"
	Kind    string `json:"kind"` // SID or STAR
	// The runways the table covers ("26L", "26R"), and for a SID where each
	// one's take-off starts: the page draws from the threshold of the
	// runway in use, or from both when it is not known.
	Runways    []string    `json:"runways"`
	Thresholds []Threshold `json:"thresholds,omitempty"`
	Legs       []Leg       `json:"legs"`
	// Complete: every point of the route placed. An incomplete route is not
	// drawn.
	Complete bool   `json:"complete"`
	Table    string `json:"table"` // the coding table's file, for the record
}

// Threshold is where a runway starts.
type Threshold struct {
	Runway string  `json:"runway"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// Leg is a segment of a route, to its point.
type Leg struct {
	Point string  `json:"point"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	// Named: a reporting point or a navaid, drawn with its name; otherwise
	// a technical point (PG271) the line passes through.
	Named bool   `json:"named,omitempty"`
	Path  string `json:"path"` // the path terminator: TF, CF, DF, IF…
	// The constraints at the point, as the table gives them: "FL060" or
	// "4000" (feet). Empty when none, or when not read without doubt.
	MinAlt string `json:"min_alt,omitempty"`
	MaxAlt string `json:"max_alt,omitempty"`
	MaxKt  int    `json:"max_kt,omitempty"`
	placed bool
	read   bool // the constraint columns read without doubt
	line   int  // the table's line, for the record
}

// RoutesNote is what the reading of the routes says: how many are whole,
// and how the constraints were read.
type RoutesNote struct {
	Routes            int `json:"routes"`
	Complete          int `json:"complete"`
	Legs              int `json:"legs"`
	ConstraintRows    int `json:"constraint_rows"`    // legs whose constraint columns were read
	ConstraintDoubted int `json:"constraint_doubted"` // legs whose constraints were left out
}

var (
	reLegLine   = regexp.MustCompile(`(?:^|\s)(IF|TF|CF|DF|FA|FC|FD|FM|CA|CD|CI|CR|RF|AF|VA|VD|VI|VM|VR|HA|HF|HM)\s+([A-Z][A-Z0-9]{1,4}|-)(?:\s|$)`)
	reProcHead  = regexp.MustCompile(`^\s{0,24}([A-Z]{2,5} ?\d[A-Z])(\s+\(.*\))?\s*$`)
	reRunwayRow = regexp.MustCompile(`^\s*RWY\s?(\d{2}[LRC]?)\b`)
	reAltValue  = regexp.MustCompile(`^(?:FL\s?(\d{3})|(\d{3,5}))$`)
	reKtValue   = regexp.MustCompile(`^(\d{3})$`)
	reTableRwys = regexp.MustCompile(`RWY(\d{2}[LRC]?)(?:-(\d{2}[LRC]?))?`)
)

// column is a constraint column of a table: its name and the centre of its
// header, in characters.
type column struct {
	name   string
	centre float64
}

// tableColumns finds the centres of the value columns in a table's header:
// fly-over, the two directions, distance, turn, the two altitudes, the
// speed and the accuracy, in that order; nil when the header is not whole.
func tableColumns(lines []string) []column {
	var fl, nm []float64
	var kt, turn, over, spec float64 = -1, -1, -1, -1
	for _, l := range lines {
		r := []rune(l)
		find := func(word string) []float64 {
			var out []float64
			s := string(r)
			for i := 0; ; {
				j := strings.Index(s[i:], word)
				if j < 0 {
					return out
				}
				start := utf8.RuneCountInString(s[:i+j])
				out = append(out, float64(start)+float64(utf8.RuneCountInString(word))/2)
				i += j + len(word)
			}
		}
		if c := find("(FL or AMSL ft)"); len(c) == 2 {
			fl = c
		}
		// The distance's (NM) and the accuracy's may sit on two lines.
		for _, c := range find("(NM)") {
			dup := false
			for _, x := range nm {
				dup = dup || math.Abs(x-c) < 3
			}
			if !dup {
				nm = append(nm, c)
			}
		}
		// The last column is the navigation accuracy (NM) or, in some
		// tables, the navigation specification ("RNAV 1").
		if c := find("Nav Spec"); len(c) == 1 {
			spec = c[0]
		}
		if c := find("(kt)"); len(c) == 1 {
			kt = c[0]
		}
		if c := find("Turn direction"); len(c) == 1 {
			turn = c[0]
		}
		if c := find("Fly Over"); len(c) == 1 {
			over = c[0]
		}
	}
	sort.Float64s(nm)
	if len(nm) == 1 && spec >= 0 {
		nm = append(nm, spec)
	}
	if len(fl) != 2 || len(nm) != 2 || kt < 0 || turn < 0 || over < 0 {
		return nil
	}
	return []column{
		{"over", over}, {"dist", nm[0]}, {"turn", turn},
		{"min", fl[0]}, {"max", fl[1]}, {"kt", kt}, {"acc", nm[1]},
	}
}

// token is a word of a line and where its middle is.
type token struct {
	text   string
	centre float64
}

func tokens(line string) []token {
	var out []token
	r := []rune(line)
	for i := 0; i < len(r); {
		if r[i] == ' ' {
			i++
			continue
		}
		j := i
		for j < len(r) && r[j] != ' ' {
			j++
		}
		out = append(out, token{string(r[i:j]), float64(i+j) / 2})
		i = j
	}
	// "FL 100" is one value.
	for k := 0; k+1 < len(out); k++ {
		if out[k].text == "FL" && reKtValue.MatchString(out[k+1].text) {
			out[k] = token{"FL" + out[k+1].text, (out[k].centre + out[k+1].centre) / 2}
			out = append(out[:k+1], out[k+2:]...)
		}
	}
	return out
}

// constraints reads a leg's altitude and speed constraints: each value goes
// to the column whose header it sits under; a value nearly as close to a
// second column, or two values under one column, and the leg's constraints
// are left out (ok false).
func constraints(after []token, cols []column) (min, max string, kt int, ok bool) {
	if cols == nil {
		return "", "", 0, false
	}
	taken := map[string]string{}
	for _, t := range after {
		best, second := -1, -1
		for i, c := range cols {
			d := math.Abs(t.centre - c.centre)
			if best < 0 || d < math.Abs(t.centre-cols[best].centre) {
				second, best = best, i
			} else if second < 0 || d < math.Abs(t.centre-cols[second].centre) {
				second = i
			}
		}
		// Only the constraint columns matter; elsewhere the direction values
		// sit between columns this does not name.
		name := cols[best].name
		if name != "min" && name != "max" && name != "kt" {
			continue
		}
		d1 := math.Abs(t.centre - cols[best].centre)
		d2 := math.Abs(t.centre - cols[second].centre)
		if d2-d1 < 3 {
			return "", "", 0, false
		}
		if _, dup := taken[name]; dup {
			return "", "", 0, false
		}
		taken[name] = t.text
	}
	alt := func(v string) (string, bool) {
		if v == "" || v == "-" {
			return "", true
		}
		m := reAltValue.FindStringSubmatch(v)
		if m == nil {
			return "", false
		}
		if m[1] != "" {
			return "FL" + m[1], true
		}
		return m[2], true
	}
	var ok1, ok2 bool
	if min, ok1 = alt(taken["min"]); !ok1 {
		return "", "", 0, false
	}
	if max, ok2 = alt(taken["max"]); !ok2 {
		return "", "", 0, false
	}
	if v := taken["kt"]; v != "" && v != "-" {
		n, err := strconv.Atoi(v)
		if err != nil || !reKtValue.MatchString(v) {
			return "", "", 0, false
		}
		kt = n
	}
	return min, max, kt, true
}

// parseCodingTable reads one coding table's text (pdftotext -layout).
func parseCodingTable(text, airport, kind, table string, note *RoutesNote) []Route {
	lines := strings.Split(text, "\n")
	var runways []string
	if m := reTableRwys.FindStringSubmatch(table); m != nil {
		runways = append(runways, m[1])
		if m[2] != "" {
			runways = append(runways, m[2])
		}
	}
	var routes []Route
	var cols []column
	var recent []string
	var cur *Route
	for ln, line := range lines {
		// A header comes before the procedures of each page: its column
		// names span the few lines above its last one.
		recent = append(recent, line)
		if len(recent) > 6 {
			recent = recent[1:]
		}
		if strings.Contains(line, "(FL or AMSL ft)") || strings.Contains(line, "(kt)") {
			if c := tableColumns(recent); c != nil {
				cols = c
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "RWY") || strings.HasPrefix(trimmed, "HLDG") {
			if m := reRunwayRow.FindStringSubmatch(line); m == nil || reLegLine.FindStringSubmatchIndex(line) == nil {
				continue
			}
		}
		if h := reProcHead.FindStringSubmatch(line); h != nil {
			name := h[1]
			if !strings.Contains(name, " ") {
				name = name[:len(name)-2] + " " + name[len(name)-2:]
			}
			routes = append(routes, Route{Airport: airport, Name: name, Kind: kind, Runways: runways, Table: table})
			cur = &routes[len(routes)-1]
			continue
		}
		loc := reLegLine.FindStringSubmatchIndex(line)
		if loc == nil || cur == nil {
			continue
		}
		path, point := line[loc[2]:loc[3]], line[loc[4]:loc[5]]
		if point == "-" {
			continue // a leg to an altitude or a heading: no point to draw
		}
		leg := Leg{Point: point, Path: path, line: ln}
		var after []token
		for _, t := range tokens(line) {
			if t.centre > float64(utf8.RuneCountInString(line[:loc[5]])) {
				after = append(after, t)
			}
		}
		note.Legs++
		if min, max, kt, ok := constraints(after, cols); ok {
			leg.MinAlt, leg.MaxAlt, leg.MaxKt, leg.read = min, max, kt, true
			note.ConstraintRows++
		} else {
			note.ConstraintDoubted++
		}
		cur.Legs = append(cur.Legs, leg)
	}
	out := routes[:0]
	for _, r := range routes {
		if len(r.Legs) > 0 {
			out = append(out, r)
		}
	}
	return out
}

var reTerminal = regexp.MustCompile(`^\s*([A-Z]{2}[0-9]{3}|[A-Z0-9]{2,5})\s+(\d{2})°(\d{2})['’](\d{2}(?:[.,]\d+)?)(?:''|"|”)\s*([NS])\s+(\d{3})°(\d{2})['’](\d{2}(?:[.,]\d+)?)(?:''|"|”)\s*([EW])`)

// dataSheetPoints reads an airport's DATA sheet (pdftotext -layout): the
// technical points with their coordinates; the named ones refer to ENR 4.4.
func dataSheetPoints(text string) map[string][2]float64 {
	out := map[string][2]float64{}
	for _, line := range strings.Split(text, "\n") {
		m := reTerminal.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out[m[1]] = [2]float64{sexa(m[2], m[3], m[4], m[5]), sexa(m[6], m[7], m[8], m[9])}
	}
	return out
}

func sexa(d, m, s, h string) float64 {
	dd, _ := strconv.Atoi(d)
	mm, _ := strconv.Atoi(m)
	ss, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	v := float64(dd) + float64(mm)/60 + ss/3600
	if h == "S" || h == "W" {
		v = -v
	}
	return v
}

var reNavaid = regexp.MustCompile(`\b([A-Z]{2,3}) \d{2,4}(?:[.,]\d+)? ?(?:MHz|kHz)(?:.{0,120}?)(\d{2})°(\d{2})'(\d{2}(?:[.,]\d+)?)"([NS]) (\d{3})°(\d{2})'(\d{2}(?:[.,]\d+)?)"([EW])`)

// navaids reads ENR 4.1: each radio navaid's identifier and position.
func navaids(page string) map[string][2]float64 {
	out := map[string][2]float64{}
	for _, m := range reNavaid.FindAllStringSubmatch(pageText(page), -1) {
		if _, ok := out[m[1]]; !ok {
			out[m[1]] = [2]float64{sexa(m[2], m[3], m[4], m[5]), sexa(m[6], m[7], m[8], m[9])}
		}
	}
	return out
}

var reThreshold = regexp.MustCompile(`\b(\d{2}[LRC]?) \d{3}[.,]\d{2}[\s\x{00a0}]+\(\d{3}\) .{0,160}?(\d{2})°(\d{2})'(\d{2}(?:[.,]\d+)?)"([NS]) (\d{3})°(\d{2})'(\d{2}(?:[.,]\d+)?)"([EW])`)

// thresholds reads an AD 2 page's runway table (AD 2.12): where each
// runway's threshold is.
func thresholds(page string) map[string][2]float64 {
	// The pattern is the table's row (designator, true bearing, magnetic
	// one in brackets, then the threshold): no other table of the page
	// has it, and the page's contents list names AD 2.12 too.
	t := pageText(page)
	out := map[string][2]float64{}
	for _, m := range reThreshold.FindAllStringSubmatch(t, -1) {
		if _, ok := out[m[1]]; !ok {
			out[m[1]] = [2]float64{sexa(m[2], m[3], m[4], m[5]), sexa(m[6], m[7], m[8], m[9])}
		}
	}
	return out
}

// placeRoutes reads what places the routes' points (an airport's DATA
// sheets, ENR 4.1, the AD 2.12 thresholds), places them, merges a route the
// tables give once per runway, and keeps them in the list.
func (m *Maker) placeRoutes(ctx context.Context, n *Names, routes []Route, note *RoutesNote, base, folder string, airports []string, pages map[string]string, tool string) {
	technical := map[string][2]float64{}
	thr := map[string]map[string][2]float64{}
	refs := map[string]bool{}
	for _, a := range airports {
		page := pages[fmt.Sprintf(pageAirport, a)]
		thr[a] = thresholds(page)
		seen := map[string]bool{}
		for _, mm := range reDataSheet.FindAllStringSubmatch(page, -1) {
			rel := mm[1]
			if seen[rel] {
				continue
			}
			seen[rel] = true
			p := filepath.Join(folder, filepath.FromSlash(rel))
			if _, err := os.Stat(p); err != nil {
				if err := m.fetch(ctx, base+"/"+rel, p); err != nil {
					continue // its routes stay incomplete, and are not drawn
				}
			}
			out, err := exec.CommandContext(ctx, tool, "-layout", p, "-").Output()
			if err != nil {
				continue
			}
			for k, v := range dataSheetPoints(string(out)) {
				technical[k] = v
			}
			for _, r := range reNavaidRef.FindAllStringSubmatch(string(out), -1) {
				refs[r[2]] = true
			}
		}
	}
	// The navaids a DATA sheet places at another aerodrome ("BOV REF AD2
	// LFOB.19"): that aerodrome's AD 2 page, its navaids table.
	nav := map[string][2]float64{}
	for a := range refs {
		page, ok := pages[fmt.Sprintf(pageAirport, a)]
		if !ok {
			name := fmt.Sprintf(pageAirport, a)
			p := filepath.Join(folder, name)
			if _, err := os.Stat(p); err != nil {
				if err := m.fetch(ctx, base+"/"+name, p); err != nil {
					continue
				}
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			page = string(raw)
		}
		for k, v := range navaids(page) {
			nav[k] = v
		}
	}
	p := filepath.Join(folder, pageNavaids)
	if _, err := os.Stat(p); err != nil {
		_ = m.fetch(ctx, base+"/"+pageNavaids, p)
	}
	if raw, err := os.ReadFile(p); err == nil {
		for k, v := range navaids(string(raw)) {
			nav[k] = v
		}
	}
	placeRoutes(routes, n.allPoints, nav, technical, thr)
	routes = mergeRunways(routes)
	note.Routes = len(routes)
	for _, r := range routes {
		if r.Complete {
			note.Complete++
		}
	}
	n.Routes = routes
	n.RoutesNote = note
}

// mergeRunways merges a route the tables give once per runway (a STAR has a
// table for each): same airport, name, kind and points, one route for
// all its runways.
func mergeRunways(routes []Route) []Route {
	key := func(r Route) string {
		var b strings.Builder
		b.WriteString(r.Airport + "|" + r.Kind + "|" + r.Name)
		for _, l := range r.Legs {
			b.WriteString("|" + l.Point + l.MinAlt + l.MaxAlt + strconv.Itoa(l.MaxKt))
		}
		return b.String()
	}
	at := map[string]int{}
	var out []Route
	for _, r := range routes {
		k := key(r)
		if i, ok := at[k]; ok {
			for _, rw := range r.Runways {
				if !contains(out[i].Runways, rw) {
					out[i].Runways = append(out[i].Runways, rw)
				}
			}
			out[i].Thresholds = append(out[i].Thresholds, r.Thresholds...)
			continue
		}
		at[k] = len(out)
		out = append(out, r)
	}
	for i := range out {
		sort.Strings(out[i].Runways)
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// placeRoutes gives each leg its coordinates and each SID its thresholds,
// and says which routes are whole.
func placeRoutes(routes []Route, named map[string]Point, nav, technical map[string][2]float64, thr map[string]map[string][2]float64) {
	round := func(v float64) float64 { return math.Round(v*1e5) / 1e5 }
	for i := range routes {
		r := &routes[i]
		r.Complete = true
		for j := range r.Legs {
			l := &r.Legs[j]
			if p, ok := named[l.Point]; ok {
				l.Lat, l.Lon, l.Named, l.placed = p.Lat, p.Lon, true, true
			} else if c, ok := nav[l.Point]; ok {
				l.Lat, l.Lon, l.Named, l.placed = c[0], c[1], true, true
			} else if c, ok := technical[l.Point]; ok {
				l.Lat, l.Lon, l.placed = c[0], c[1], true
			}
			l.Lat, l.Lon = round(l.Lat), round(l.Lon)
			if !l.placed {
				r.Complete = false
			}
		}
		if r.Kind == "SID" {
			for _, rw := range r.Runways {
				if c, ok := thr[r.Airport][rw]; ok {
					r.Thresholds = append(r.Thresholds, Threshold{Runway: rw, Lat: round(c[0]), Lon: round(c[1])})
				}
			}
			if len(r.Thresholds) != len(r.Runways) {
				r.Complete = false
			}
		}
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Airport != routes[j].Airport {
			return routes[i].Airport < routes[j].Airport
		}
		return routes[i].Name < routes[j].Name
	})
}

// pageNavaids is ENR 4.1, read for the routes alone.
const pageNavaids = "FR-ENR-4.1-fr-FR.html"

var reNavaidRef = regexp.MustCompile(`([A-Z]{2,3})\s+REF AD2(?: -)? (LF[A-Z]{2})\.\s?19`)

var reDataSheet = regexp.MustCompile(`href="(Cartes/(LF[A-Z]{2})/AD_2_LF[A-Z]{2}_DATA_0\d\.pdf)"`)

func routesNoteText(n RoutesNote) string {
	return fmt.Sprintf("%d of %d published routes whole; constraints read on %d of %d segments, %d left out",
		n.Complete, n.Routes, n.ConstraintRows, n.Legs, n.ConstraintDoubted)
}
