package phraseology

import "strings"

// Callsigns with letters in their flight part, and airline names heard roughly.
//
// Most flight numbers over Paris are not numbers: on the reference capture of
// 24/09/2026, 59% of 1 408 callsigns were letters with fewer than three digits
// -- AFR11NQ, VLG7UE, EZY36VJ (docs-fr/05-decisions.md, Q46). The matcher keys
// on three digits or more, so those aircraft could only ever be reached through
// a letter bonus added to other evidence. Both rules below are off by default:
// they are there to be measured against a shuffled sky before anything uses them.

// alnumGroup is a flight part as spoken: digits then spelled letters, "seven
// uniform echo" read as "7UE".
type alnumGroup struct {
	text string
	word [2]int // token range
}

// alnumGroups finds every spoken number immediately followed by spelled letters.
// A number alone is left to the digit rules, and letters alone to letterGroups.
func alnumGroups(toks []string) []alnumGroup {
	var out []alnumGroup
	for i := 0; i < len(toks); i++ {
		if !opensNumber(toks[i]) {
			continue
		}
		digits, end := readNumber(toks, i)
		if end <= i || digits == "" || strings.Contains(digits, ".") {
			continue
		}
		var letters []byte
		j := end
		for j < len(toks) {
			c, ok := isLetterWord(toks[j])
			if !ok {
				break
			}
			letters = append(letters, c)
			j++
		}
		if len(letters) > 0 {
			out = append(out, alnumGroup{digits + string(letters), [2]int{i, j}})
		}
		i = end - 1
	}
	return out
}

// alnumScore scores a spoken flight part against an aircraft's. The whole flight
// part, three characters or more, weighs as much as three exact digits: 7UE is
// one of 6 760 digit-letter-letter combinations, where 350 is one of 1 000. A
// flight part missing only its last letter -- "seven uniform" for 7UE, the
// letter lost to the radio -- weighs less than the 0.6 floor, and only lands
// with a corroboration: an airline name, an altitude.
func alnumScore(spoken, flight string) (float64, string) {
	switch {
	case len(flight) >= 3 && spoken == flight:
		return 0.9, "flight part exact"
	case len(spoken) >= 2 && len(flight) == len(spoken)+1 && strings.HasPrefix(flight, spoken):
		return 0.5, "flight part, last letter missing"
	}
	return 0, ""
}

// operatorsNear finds airline names heard approximately, among the operators
// actually in the sky: "welling" for VUELING. Searching the whole of
// airlines.dat would find a near-namesake for most words; searching the forty
// operators present finds the one the controller could have meant.
func (m *Matcher) operatorsNear(toks []string, present map[string]bool) map[string][][2]int {
	out := map[string][][2]int{}
	for i, w := range toks {
		if len(w) < 5 {
			continue // short words are near too many names
		}
		for key, code := range m.telephony {
			if !present[code] || strings.Contains(key, " ") {
				continue
			}
			limit := 1
			if len(key) >= 7 {
				limit = 2
			}
			d := editDistance(w, key)
			if d == 0 {
				continue // heard exactly: operatorsIn already has it
			}
			if d <= limit || soundsLike(w, key) {
				out[code] = append(out[code], [2]int{i, i + 1})
			}
		}
	}
	return out
}

// soundsLike compares two words by their consonants, the part that survives a
// radio: vowels dropped, W read as V, doubled letters merged. "welling" and
// "vueling" both give "vlng" though three letters apart. Three consonants at
// least, or too many words would sound alike.
func soundsLike(a, b string) bool {
	ka, kb := consonants(a), consonants(b)
	return len(ka) >= 3 && ka == kb
}

func consonants(w string) string {
	var out []byte
	for i := 0; i < len(w); i++ {
		c := w[i]
		switch c {
		case 'a', 'e', 'i', 'o', 'u', 'y':
			continue
		case 'w':
			c = 'v'
		}
		if n := len(out); n > 0 && out[n-1] == c {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// Rules are the association rules that can be changed while the server runs,
// from the settings panel.
type Rules struct {
	Letters         bool // AlnumCallsigns
	ApproxOperators bool // FuzzyOperators
	MinDigits       int  // 0 keeps the matcher's own
	OneDigitOff     bool // FuzzyDigits
	ContextLetters  bool
	ContextDigits   bool
	ContextNames    bool
}

// WithRules returns a copy of the matcher applying r. The copy shares the
// airline tables, which are only ever read.
func (m *Matcher) WithRules(r Rules) *Matcher {
	c := *m
	c.AlnumCallsigns, c.FuzzyOperators, c.FuzzyDigits = r.Letters, r.ApproxOperators, r.OneDigitOff
	c.ContextLetters, c.ContextDigits, c.ContextNames = r.ContextLetters, r.ContextDigits, r.ContextNames
	if r.MinDigits > 0 {
		c.MinDigits = r.MinDigits
	}
	return &c
}
