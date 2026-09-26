package phraseology

import "strings"

// Abbreviated callsigns in an exchange already under way.
//
// After first contact, pilot and controller shorten the callsign: "Sierra
// Bravo" for AFR23SB, "five four" for LBT554, "Lufthansa" alone. The matcher
// already remembers the aircraft heard on the frequency in the last minutes,
// but only to break ties. On the 24/09 transmissions left without an aircraft,
// the last letters of an aircraft matched just before on the same frequency
// appear 17 times against 1.3 by chance (another frequency's aircraft in its
// place); the last two digits 22 against 13, the airline name alone 90 against
// 51 (docs-fr/05-decisions.md, Q46). The three rules below are off by default,
// to be measured each on its own.

const contextScore = 0.7 // above the 0.6 floor: the exchange is the evidence

// contextScores scores the aircraft heard recently on this frequency from an
// abbreviated callsign. It returns, per callsign, the best score and its reason.
func (m *Matcher) contextScores(r Result, fleet []Aircraft, heard map[string]bool,
	spokenOperators map[string][][2]int) map[string]scoredReason {
	out := map[string]scoredReason{}
	if !m.ContextLetters && !m.ContextNames && !m.ContextDigits {
		return out
	}
	var recent []Aircraft
	byOperator := map[string]int{}
	byTail := map[string]int{}
	for _, ac := range fleet {
		if !heard[ac.Callsign] {
			continue
		}
		recent = append(recent, ac)
		prefix, digits, _ := splitCallsign(ac.Callsign)
		byOperator[prefix]++
		if len(digits) >= 3 {
			byTail[digits[len(digits)-2:]]++
		}
	}
	set := func(cs string, s float64, why string) {
		if s > out[cs].score {
			out[cs] = scoredReason{s, why}
		}
	}
	for _, ac := range recent {
		prefix, digits, letters := splitCallsign(ac.Callsign)
		if m.ContextLetters && letters != "" {
			for _, g := range r.Letters {
				if len(g) >= 2 && strings.HasSuffix(letters, g) {
					set(ac.Callsign, contextScore, "last letters of an aircraft just heard")
				}
			}
		}
		if m.ContextNames && len(spokenOperators[prefix]) > 0 && byOperator[prefix] == 1 {
			set(ac.Callsign, contextScore, "only aircraft of the named airline just heard")
		}
		if m.ContextDigits && len(digits) >= 3 && byTail[digits[len(digits)-2:]] == 1 {
			for _, v := range r.Values {
				if v.Role == RoleCallsign && len(v.Digits) == 2 && v.Digits == digits[len(digits)-2:] {
					set(ac.Callsign, contextScore, "last two digits of an aircraft just heard")
				}
			}
		}
	}
	return out
}

type scoredReason struct {
	score float64
	why   string
}
