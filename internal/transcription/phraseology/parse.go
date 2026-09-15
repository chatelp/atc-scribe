package phraseology

import (
	"fmt"
	"strconv"
	"strings"
)

// Role is what a spoken number turned out to mean.
type Role string

const (
	RoleFlightLevel Role = "flight_level"
	RoleAltitude    Role = "altitude"
	RoleHeading     Role = "heading"
	RoleSpeed       Role = "speed"
	RoleFrequency   Role = "frequency"
	RoleRunway      Role = "runway"
	RoleQNH         Role = "qnh"
	RoleSquawk      Role = "squawk"
	RoleCallsign    Role = "callsign"
	RoleUnknown     Role = "unknown"
)

// Value is one number found in a transmission, with the role its context gave it.
type Value struct {
	Role   Role
	Digits string // as spoken, e.g. "350"
	Text   string // normalised for display, e.g. "FL350", "121.9", "27L"
	Word   int    // token index where the value started
}

// Speaker is who transmitted.
type Speaker string

const (
	SpeakerATC     Speaker = "ATC"
	SpeakerPilot   Speaker = "PILOT"
	SpeakerUnknown Speaker = ""
)

// Clearance is a takeoff, landing or approach clearance, in the closed set that
// upstream's post-processing prompt enumerates.
type Clearance struct {
	Type   string // "takeoff" | "landing" | "approach"
	Text   string
	Runway string
}

// Result is everything the grammar could establish about one transmission.
type Result struct {
	Raw        string
	Normalized string
	Values     []Value
	Letters    []string // NATO letter groups, e.g. "FKP" — light-aircraft callsigns
	Speaker    Speaker
	Clearances []Clearance
}

// roleKeywords maps the phrase that introduces a number to the role it confers.
// Longest phrases are matched first, so "flight level" wins over "level".
var roleKeywords = []struct {
	words []string
	role  Role
}{
	{[]string{"flight", "level"}, RoleFlightLevel},
	{[]string{"descend", "level"}, RoleFlightLevel},
	{[]string{"climb", "level"}, RoleFlightLevel},
	{[]string{"level"}, RoleFlightLevel},
	{[]string{"altitude"}, RoleAltitude},
	{[]string{"height"}, RoleAltitude},
	{[]string{"heading"}, RoleHeading},
	{[]string{"turn", "left", "heading"}, RoleHeading},
	{[]string{"turn", "right", "heading"}, RoleHeading},
	{[]string{"speed"}, RoleSpeed},
	{[]string{"knots"}, RoleSpeed},
	{[]string{"contact"}, RoleFrequency},
	{[]string{"frequency"}, RoleFrequency},
	{[]string{"monitor"}, RoleFrequency},
	{[]string{"runway"}, RoleRunway},
	{[]string{"qnh"}, RoleQNH},
	{[]string{"squawk"}, RoleSquawk},
	{[]string{"transponder"}, RoleSquawk},

	// Phraséologie française. Le contrôle français emploie les mêmes rôles avec
	// d'autres mots, et cette station les entend tous les jours.
	{[]string{"niveau", "de", "vol"}, RoleFlightLevel},
	{[]string{"niveau"}, RoleFlightLevel},
	{[]string{"altitude"}, RoleAltitude},
	{[]string{"cap"}, RoleHeading},
	{[]string{"vitesse"}, RoleSpeed},
	{[]string{"piste"}, RoleRunway},
	{[]string{"fréquence"}, RoleFrequency},
	{[]string{"frequence"}, RoleFrequency},
	{[]string{"contactez"}, RoleFrequency},
	{[]string{"transpondeur"}, RoleSquawk},
	{[]string{"affichez"}, RoleSquawk},
}

// atcPhrases are said by controllers and not echoed as instructions by pilots.
var atcPhrases = [][]string{
	{"cleared", "to", "land"}, {"cleared", "for", "takeoff"}, {"cleared", "for", "take", "off"},
	{"line", "up", "and", "wait"}, {"hold", "short"}, {"continue", "approach"},
	{"go", "around"}, {"radar", "contact"}, {"identified"}, {"no", "delay"},
	{"report", "when"}, {"break", "break"}, {"traffic", "is"},
}

// pilotPhrases are read-backs and requests.
var pilotPhrases = [][]string{
	{"request"}, {"with", "you"}, {"wilco"}, {"we", "are"}, {"we", "will"},
	{"ready", "for", "departure"}, {"ready", "for", "takeoff"}, {"good", "day"},
	{"passing"}, {"leaving"}, {"on", "track"}, {"looking", "for"}, {"say", "again"},
}

// Parse extracts every fact the closed vocabulary of ICAO phraseology allows.
//
// It is deliberately keyword-driven rather than grammatical: transcription output
// is noisy, and anchoring on "flight level" or "runway" survives garbage on either
// side of the anchor, where a full parse would fail on the whole sentence.
func Parse(text string) Result {
	toks := tokenize(text)
	res := Result{Raw: text}

	used := make([]bool, len(toks))
	for i := 0; i < len(toks); i++ {
		kw, role, n := matchKeyword(toks, i)
		if n == 0 {
			continue
		}
		digits, end := readNumber(toks, i+n)
		if digits == "" {
			continue
		}
		if role == RoleFlightLevel && !plausibleFlightLevel(digits) {
			role = RoleUnknown
		}
		v := Value{Role: role, Digits: digits, Word: i}
		v.Text = render(role, digits, toks, end)
		res.Values = append(res.Values, v)
		for j := i; j < end; j++ {
			used[j] = true
		}
		i = end - 1
		_ = kw
	}

	res.Letters = letterGroups(toks, used)
	res.Values = append(res.Values, looseNumbers(toks, used)...)
	res.Speaker = speakerOf(toks)
	res.Clearances = clearances(toks, res.Values)
	res.Normalized = normalize(text, res.Values)
	return res
}

// matchKeyword returns the role introduced at position i, and how many tokens it spans.
func matchKeyword(toks []string, i int) (string, Role, int) {
	best, bestRole, bestN := "", RoleUnknown, 0
	for _, kw := range roleKeywords {
		if len(kw.words) > bestN && matchAt(toks, i, kw.words) {
			best, bestRole, bestN = strings.Join(kw.words, " "), kw.role, len(kw.words)
		}
	}
	return best, bestRole, bestN
}

func matchAt(toks []string, i int, words []string) bool {
	if i+len(words) > len(toks) {
		return false
	}
	for k, w := range words {
		if toks[i+k] != w {
			return false
		}
	}
	return true
}

// readNumber consumes spoken digits and grouped forms from position i, returning
// the digit string and the index just past it.
func readNumber(toks []string, i int) (string, int) {
	var b strings.Builder
	j := i
	for j < len(toks) {
		w := toks[j]
		if d, ok := isDigitWord(w); ok {
			// An ambiguous word only counts as a digit when another digit follows,
			// so a trailing preposition does not become a trailing digit.
			if homophones[w] && !digitFollows(toks, j+1) {
				break
			}
			b.WriteString(d)
			j++
			continue
		}
		if n, ok := groupedNumbers[w]; ok {
			b.WriteString(strconv.Itoa(n))
			j++
			continue
		}
		if n, ok := frenchGrouped[w]; ok {
			b.WriteString(strconv.Itoa(n))
			j++
			continue
		}
		if w == wordMille && b.Len() > 0 {
			b.WriteString("000")
			j++
			continue
		}
		if w == wordHundred && b.Len() > 0 {
			b.WriteString("00")
			j++
			continue
		}
		if w == wordThousand && b.Len() > 0 {
			b.WriteString("000")
			j++
			continue
		}
		// "decimal" and "point" join the halves of a frequency.
		if (w == "decimal" || w == "point") && b.Len() > 0 {
			b.WriteString(".")
			j++
			continue
		}
		if len(w) > 0 && w[0] >= '0' && w[0] <= '9' {
			b.WriteString(w)
			j++
			continue
		}
		break
	}
	return b.String(), j
}

// digitFollows reports whether the token at i reads as a digit in its own right.
func digitFollows(toks []string, i int) bool {
	if i >= len(toks) {
		return false
	}
	w := toks[i]
	if homophones[w] {
		return false
	}
	if _, ok := isDigitWord(w); ok {
		return true
	}
	_, ok := groupedNumbers[w]
	return ok
}

// render turns raw digits into the form a controller would write.
func render(role Role, digits string, toks []string, end int) string {
	switch role {
	case RoleFlightLevel:
		return "FL" + strings.TrimPrefix(digits, "0")
	case RoleHeading:
		if len(digits) <= 3 {
			return fmt.Sprintf("%03s", digits)
		}
		return digits
	case RoleFrequency:
		if !strings.Contains(digits, ".") && len(digits) >= 5 {
			return digits[:3] + "." + digits[3:]
		}
		return digits
	case RoleRunway:
		side := ""
		if end < len(toks) {
			switch toks[end] {
			case "left":
				side = "L"
			case "right":
				side = "R"
			case "center", "centre":
				side = "C"
			}
		}
		return digits + side
	default:
		return digits
	}
}

// trailingRoles are the words that give a number its role by following it rather
// than preceding it. Terminal control speaks altitudes as "four thousand feet",
// never "altitude four thousand" — measured: only 8 of 409 transmissions from an
// approach sector carried a flight level, while feet were everywhere.
var trailingRoles = map[string]Role{
	"feet": RoleAltitude, "foot": RoleAltitude, "ft": RoleAltitude,
	"knots": RoleSpeed, "knot": RoleSpeed,
	"degrees": RoleHeading,
	"pieds":   RoleAltitude, "noeuds": RoleSpeed, "nœuds": RoleSpeed,
	"degrés": RoleHeading, "degres": RoleHeading,
}

// letterGroups collects runs of NATO letters not already consumed as a value.
// Three or more in a row is a light-aircraft callsign such as F-GKPV read out.
func letterGroups(toks []string, used []bool) []string {
	var out []string
	var cur []byte
	flush := func() {
		if len(cur) >= 3 {
			out = append(out, string(cur))
		}
		cur = cur[:0]
	}
	for i, w := range toks {
		if used[i] {
			flush()
			continue
		}
		if c, ok := isLetterWord(w); ok {
			cur = append(cur, c)
			continue
		}
		flush()
	}
	flush()
	return out
}

// opensNumber reports whether a token can start a number.
func opensNumber(w string) bool {
	if _, ok := isDigitWord(w); ok {
		return true
	}
	if _, ok := groupedNumbers[w]; ok {
		return true
	}
	if _, ok := frenchGrouped[w]; ok {
		return true
	}
	return isNumeric(w)
}

func isNumeric(w string) bool {
	if w == "" {
		return false
	}
	for i := 0; i < len(w); i++ {
		if w[i] < '0' || w[i] > '9' {
			return false
		}
	}
	return true
}

// looseNumbers picks up digit runs with no keyword in front. They are the callsign
// candidates: in ICAO phraseology a bare number group is almost always the flight
// number, everything else being introduced by its role word.
func looseNumbers(toks []string, used []bool) []Value {
	var out []Value
	for i := 0; i < len(toks); i++ {
		if used[i] {
			continue
		}
		// A run may open on a spelled digit, a grouped form, or a bare numeral:
		// the models write both, sometimes in the same sentence — "manoeuvre 470
		// Maya" next to "Latour Five Eight Seven". Only opening on spelled digits
		// silently discarded every numeral the models produced.
		if !opensNumber(toks[i]) {
			continue
		}
		digits, end := readNumber(toks, i)
		if end <= i {
			// The token looked like a digit but read as none — an ambiguous word
			// with nothing numeric after it. Advance, or we never leave this index.
			continue
		}
		if end < len(toks) {
			if role, ok := trailingRoles[toks[end]]; ok {
				out = append(out, Value{Role: role, Digits: digits, Text: digits, Word: i})
				i = end
				continue
			}
		}
		if role, text := classifyBare(digits); role != RoleUnknown {
			out = append(out, Value{Role: role, Digits: digits, Text: text, Word: i})
		} else if len(digits) >= 2 && !strings.Contains(digits, ".") {
			out = append(out, Value{Role: RoleCallsign, Digits: digits, Text: digits, Word: i})
		}
		i = end - 1
	}
	return out
}

// classifyBare recognises numbers whose value alone gives them away, with no
// keyword in front. The VHF air band is 118.000-136.975 MHz, so a five or six
// digit group starting in that range is a frequency handoff and not a flight
// number — measured on 125.933, where "one three two five zero five" was counted
// three times in forty transmissions as an aircraft.
func classifyBare(digits string) (Role, string) {
	if strings.Contains(digits, ".") {
		if mhz, err := strconv.ParseFloat(digits, 64); err == nil && mhz >= 118 && mhz < 137 {
			return RoleFrequency, digits
		}
		return RoleUnknown, ""
	}
	if n := len(digits); n == 5 || n == 6 {
		if head, err := strconv.Atoi(digits[:3]); err == nil && head >= 118 && head <= 136 {
			return RoleFrequency, digits[:3] + "." + digits[3:]
		}
	}
	return RoleUnknown, ""
}

// plausibleFlightLevel rejects levels outside the usable band. FL26 came from
// "level to six points" in a real transmission: a preposition and a word that
// is not a digit at all.
func plausibleFlightLevel(digits string) bool {
	n, err := strconv.Atoi(digits)
	return err == nil && n >= 30 && n <= 660
}

func speakerOf(toks []string) Speaker {
	atc, pilot := 0, 0
	for i := range toks {
		for _, p := range atcPhrases {
			if matchAt(toks, i, p) {
				atc++
			}
		}
		for _, p := range pilotPhrases {
			if matchAt(toks, i, p) {
				pilot++
			}
		}
	}
	switch {
	case atc > pilot:
		return SpeakerATC
	case pilot > atc:
		return SpeakerPilot
	default:
		return SpeakerUnknown
	}
}

// clearancePatterns is the closed set upstream's prompt enumerates. Conditional
// clearances ("cleared to land number two") are deliberately not matched.
var clearancePatterns = []struct {
	words []string
	kind  string
}{
	{[]string{"cleared", "to", "land"}, "landing"},
	{[]string{"cleared", "for", "landing"}, "landing"},
	{[]string{"cleared", "for", "takeoff"}, "takeoff"},
	{[]string{"cleared", "for", "take", "off"}, "takeoff"},
	{[]string{"cleared", "to", "go"}, "takeoff"},
	{[]string{"cleared", "for", "the", "approach"}, "approach"},
	{[]string{"cleared", "for", "approach"}, "approach"},
	{[]string{"cleared", "for", "the", "ils", "approach"}, "approach"},
	{[]string{"cleared", "for", "the", "visual", "approach"}, "approach"},
	{[]string{"cleared", "for", "the", "rnav", "approach"}, "approach"},
}

func clearances(toks []string, values []Value) []Clearance {
	var out []Clearance
	for i := range toks {
		for _, p := range clearancePatterns {
			if !matchAt(toks, i, p.words) {
				continue
			}
			c := Clearance{Type: p.kind, Text: strings.Join(toks[i:min(i+len(p.words)+3, len(toks))], " ")}
			// The runway belongs to this clearance if one was named nearby.
			for _, v := range values {
				if v.Role == RoleRunway && v.Word > i-6 && v.Word < i+8 {
					c.Runway = v.Text
					break
				}
			}
			out = append(out, c)
		}
	}
	return out
}

// normalize rewrites spoken numbers as digits, which is what upstream's
// post-processing prompt asks a language model to do.
func normalize(text string, values []Value) string {
	toks := tokenize(text)
	out := make([]string, 0, len(toks))
	skip := map[int]bool{}
	byWord := map[int]Value{}
	for _, v := range values {
		byWord[v.Word] = v
	}
	for i := 0; i < len(toks); i++ {
		if skip[i] {
			continue
		}
		if v, ok := byWord[i]; ok && v.Role != RoleCallsign {
			_, end := readNumber(toks, i+keywordLen(toks, i))
			out = append(out, toks[i:i+keywordLen(toks, i)]...)
			out = append(out, v.Text)
			for j := i; j < end; j++ {
				skip[j] = true
			}
			i = end - 1
			continue
		}
		out = append(out, toks[i])
	}
	return strings.Join(out, " ")
}

func keywordLen(toks []string, i int) int {
	_, _, n := matchKeyword(toks, i)
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
