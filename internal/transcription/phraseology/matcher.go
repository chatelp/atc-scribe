package phraseology

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Aircraft is one target the ADS-B receiver is currently seeing.
type Aircraft struct {
	Callsign   string // as filed, e.g. "AFR1081", "EZY36VJ"
	Hex        string
	AltitudeFt float64 // barometric altitude, 0 if unknown
	Phase      string  // "CRZ", "ARR", "DEP", "T/O", "TAX", "" if unknown
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

	// FuzzyDigits accepts a spoken number one digit away from an aircraft's.
	//
	// Off by default, and the default is measured. On the recorded capture the
	// rule adds 8 matches of which 1.5 are true and 6.5 are coincidence -- 81%
	// noise -- costing three points of precision for nothing. Live on CDG
	// approach it produced two of three wrong matches: "Air France Three Six
	// Seven", said three times over, was attached to AFR377, and "Heli One Six
	// Two" to N132QS.
	//
	// The flaw is structural rather than statistical. An off-by-one match scores
	// 0.50, under the 0.60 floor, so it only lands with a corroboration -- and a
	// single one is enough to carry it past. Over Paris, "Air France" corroborates
	// almost anything.
	FuzzyDigits bool
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

// Match returns the best aircraft for a parsed transmission, with no context.
func (m *Matcher) Match(r Result, fleet []Aircraft) (Match, bool) {
	return m.MatchWithContext(r, fleet, nil)
}

// MatchWithContext is Match with what was heard just before on the same frequency.
//
// Context is used only to choose between candidates this transmission already
// names, never to name an aircraft it does not. That restriction is not caution
// for its own sake: the shuffled control this project measures with asks whether
// an aircraft was in the sky, not whether a transmission was about it, so a match
// invented from a neighbour would pass the control while being wrong. What cannot
// be measured is not shipped.
//
// Measured ceilings on 1415 recorded transmissions: a digit group repeated inside
// one transmission occurs 15 times, 1.1%; two neighbouring transmissions sharing a
// spoken value occur essentially never, because only 4% carry a level at all.
// These rules are therefore small by nature, and they are here because they cost
// nothing and can only ever re-rank candidates that were already in the running.
func (m *Matcher) MatchWithContext(r Result, fleet []Aircraft, recent []string) (Match, bool) {
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

	// How many times each spoken number occurs in this transmission. An aircraft
	// that says its callsign three times is stating it, not being guessed at.
	repeats := map[string]int{}
	for _, v := range r.Values {
		if v.Role == RoleCallsign && len(v.Digits) >= m.MinDigits {
			repeats[v.Digits]++
		}
	}

	// The aircraft named in the last few transmissions on this frequency, used
	// only to break ties.
	heardRecently := map[string]bool{}
	for _, cs := range recent {
		heardRecently[cs] = true
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
			case m.FuzzyDigits && len(digits) >= 3 && len(v.Digits) == len(digits) && editDistance(v.Digits, digits) == 1:
				if s := 0.5; s > score {
					score, why = s, []string{"digits off by one"}
				}
			}
		}

		// A number the transmission repeats is stronger evidence than one heard
		// once, and this is the only place repetition is worth anything.
		if score > 0 && repeats[digits] > 1 {
			score += 0.15
			why = append(why, "digits repeated")
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

		// Altitude is the one corroboration where both sides are measured rather
		// than heard: a spoken flight level and a barometric altitude. When they
		// agree it is strong evidence; when they disagree by more than a level
		// change could explain, it is evidence against, which is why this may
		// subtract where a spoken airline name may not.
		if ft, ok := spokenAltitudeFt(r); ok && ac.AltitudeFt > 0 {
			switch d := absF(ac.AltitudeFt - ft); {
			case d <= 1500:
				score += 0.4
				why = append(why, "altitude agrees")
			case d >= 8000:
				score -= 0.3
				why = append(why, "altitude contradicts")
			}
		}

		// A landing or approach clearance is not issued to an aircraft in cruise.
		if hasClearance(r, "landing", "approach") && ac.Phase == "CRZ" {
			score -= 0.25
			why = append(why, "cruising, not landing")
		}
		if hasClearance(r, "takeoff") && (ac.Phase == "CRZ" || ac.Phase == "ARR") {
			score -= 0.25
			why = append(why, "airborne, not departing")
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
		// Among candidates this transmission scores equally, prefer the one the
		// frequency was just talking to. This decides nothing on its own -- both
		// were already named by the digits -- it only picks which of two equals
		// the exchange was about.
		if hi, hj := heardRecently[best[i].ac.Callsign], heardRecently[best[j].ac.Callsign]; hi != hj {
			return hi
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
			// A tie the recent exchange resolves is no longer a tie: the aircraft
			// just addressed on this frequency is the one still being addressed,
			// and the rival was never named by anything but arithmetic.
			if heardRecently[top.ac.Callsign] && !heardRecently[b.ac.Callsign] {
				continue
			}
			out.Ambiguous = true
			out.Runners = append(out.Runners, b.ac.Callsign)
		}
	}
	if heardRecently[top.ac.Callsign] {
		out.Reason += " + heard just before"
	}
	return out, true
}

// spokenAltitudeFt returns the height the transmission names, in feet, whether it
// was spoken as a flight level or in feet. Approach control uses feet, en-route
// uses levels, and the same station hears both.
func spokenAltitudeFt(r Result) (float64, bool) {
	for _, v := range r.Values {
		n, err := strconv.Atoi(v.Digits)
		if err != nil {
			continue
		}
		switch v.Role {
		case RoleFlightLevel:
			return float64(n) * 100, true
		case RoleAltitude:
			if n >= 500 && n <= 45000 {
				return float64(n), true
			}
		}
	}
	return 0, false
}

func hasClearance(r Result, kinds ...string) bool {
	for _, c := range r.Clearances {
		for _, k := range kinds {
			if c.Type == k {
				return true
			}
		}
	}
	return false
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
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
