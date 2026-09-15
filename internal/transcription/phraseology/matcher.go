package phraseology

import (
	"bufio"
	"os"
	"sort"
	"strings"
)

// Aircraft is one target the ADS-B receiver is currently seeing.
type Aircraft struct {
	Callsign string // as filed, e.g. "AFR1081", "EZY36VJ"
	Hex      string
}

// Match is the outcome of trying to attach a transmission to an aircraft.
type Match struct {
	Callsign  string
	Hex       string
	Score     float64
	Reason    string
	Ambiguous bool     // another aircraft scored just as well
	Runners   []string // the other candidates, when ambiguous
}

// Matcher attaches spoken callsigns to aircraft the receiver can actually see.
//
// The whole design rests on one measured fact: the transcription models hear
// digits reasonably and airline names badly. On this station they produced
// "Air Berlin" and "Germanwings" — carriers that stopped flying in 2017 and 2020 —
// because their fine-tuning corpora are years old. So digits carry the decision,
// and a spoken airline name can only ever *raise* a score, never lower one. A
// hallucinated operator then costs nothing; a real one helps break ties.
//
// This is also what upstream's post-processing prompt tells GPT-4o to do:
// "focus on the flight number, not airline code which may be hard to understand".
type Matcher struct {
	telephony map[string]string // spoken word -> ICAO operator code

	// MinDigits is the shortest spoken number that may stand for a flight number.
	// Measured against live ADS-B with a shuffled control, two-digit groups match
	// by arithmetic about as often as by truth; three is where signal starts.
	MinDigits int
}

// NewMatcher builds the spoken-name index from OpenFlights' airlines.dat, which
// upstream already ships in assets/. Both the company name and the radio
// telephony designator are indexed: controllers say "Air France" and "Speedbird",
// and the file holds AIRFRANS and SPEEDBIRD alongside the names.
func NewMatcher(airlinesDatPath string) (*Matcher, error) {
	m := &Matcher{telephony: map[string]string{}, MinDigits: 3}
	f, err := os.Open(airlinesDatPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		fields := splitCSV(sc.Text())
		if len(fields) < 8 {
			continue
		}
		name, icao, callsign, active := fields[1], fields[4], fields[5], fields[7]
		if len(icao) != 3 || icao == `\N` || active != "Y" {
			continue
		}
		for _, form := range []string{name, callsign} {
			key := normalizeName(form)
			if key == "" || len(key) < 3 {
				continue
			}
			// First writer wins: airlines.dat is ordered with the majors early,
			// and later duplicates are usually defunct or regional namesakes.
			if _, seen := m.telephony[key]; !seen {
				m.telephony[key] = icao
			}
		}
	}
	return m, sc.Err()
}

// Match returns the best aircraft for a parsed transmission.
func (m *Matcher) Match(r Result, fleet []Aircraft) (Match, bool) {
	spokenOperators := m.operatorsIn(r.Raw)

	// If an operator is named and that operator is actually in the sky, only its
	// aircraft are considered. This is not a contradiction of the rule that a
	// spoken name may never lower a score: an operator that is *not* present —
	// the hallucinated Air Berlins and Germanwings — constrains nothing, and the
	// digits decide alone. Measured need: "one nine zero ryan air one seven four"
	// was attached to AFR174 because the digits matched and the name was ignored.
	restrict := map[string]bool{}
	for _, ac := range fleet {
		if prefix, _, _ := splitCallsign(ac.Callsign); spokenOperators[prefix] {
			restrict[prefix] = true
		}
	}

	type scored struct {
		ac     Aircraft
		score  float64
		reason string
	}
	var best []scored

	for _, ac := range fleet {
		prefix, digits, letters := splitCallsign(ac.Callsign)
		if digits == "" && letters == "" {
			continue
		}
		if len(restrict) > 0 && !restrict[prefix] {
			continue
		}
		var score float64
		var why []string

		for _, v := range r.Values {
			if v.Role != RoleCallsign || len(v.Digits) < m.MinDigits {
				continue
			}
			switch {
			case digits != "" && v.Digits == digits:
				if s := 0.9; s > score {
					score, why = s, []string{"digits exact"}
				}
			case len(digits) >= 3 && len(v.Digits) >= 3 && strings.HasSuffix(v.Digits, digits):
				if s := 0.6; s > score {
					score, why = s, []string{"digits suffix"}
				}
			case len(digits) >= 3 && len(v.Digits) == len(digits) && editDistance(v.Digits, digits) == 1:
				if s := 0.5; s > score {
					score, why = s, []string{"digits off by one"}
				}
			}
		}

		// Letter tails such as EZY36VJ, read out as "three six victor juliett".
		if letters != "" {
			for _, g := range r.Letters {
				if strings.HasSuffix(g, letters) || strings.HasSuffix(letters, g) {
					score += 0.3
					why = append(why, "letters")
					break
				}
			}
		}

		if score == 0 {
			continue
		}
		if spokenOperators[prefix] {
			score += 0.4
			why = append(why, "operator named")
		}
		best = append(best, scored{ac, score, strings.Join(why, " + ")})
	}

	if len(best) == 0 {
		return Match{}, false
	}
	sort.Slice(best, func(i, j int) bool {
		if best[i].score != best[j].score {
			return best[i].score > best[j].score
		}
		return best[i].ac.Callsign < best[j].ac.Callsign
	})

	top := best[0]
	if top.score < 0.6 {
		return Match{}, false
	}
	out := Match{Callsign: top.ac.Callsign, Hex: top.ac.Hex, Score: top.score, Reason: top.reason}
	for _, b := range best[1:] {
		if top.score-b.score < 0.15 {
			out.Ambiguous = true
			out.Runners = append(out.Runners, b.ac.Callsign)
		}
	}
	return out, true
}

// operatorsIn finds the ICAO codes of every operator named in the text. Single
// and two-word forms are both tried: "air france" is two words, "ryanair" one.
func (m *Matcher) operatorsIn(text string) map[string]bool {
	toks := tokenize(text)
	out := map[string]bool{}
	for i := range toks {
		for n := 2; n >= 1; n-- {
			if i+n > len(toks) {
				continue
			}
			if code, ok := m.telephony[normalizeName(strings.Join(toks[i:i+n], " "))]; ok {
				out[code] = true
				break
			}
		}
	}
	return out
}

// splitCallsign breaks AFR1081 into ("AFR","1081",""), EZY36VJ into ("EZY","36","VJ").
func splitCallsign(cs string) (prefix, digits, letters string) {
	cs = strings.ToUpper(strings.TrimSpace(cs))
	i := 0
	for i < len(cs) && cs[i] >= 'A' && cs[i] <= 'Z' {
		i++
	}
	prefix = cs[:i]
	rest := cs[i:]
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	return prefix, rest[:j], rest[j:]
}

func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = minOf(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func minOf(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// splitCSV parses one airlines.dat line: comma separated, quoted fields, no header.
func splitCSV(line string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '"':
			inQuote = !inQuote
		case c == ',' && !inQuote:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	out = append(out, cur.String())
	return out
}
