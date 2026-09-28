package phraseology

import "sort"

// Anatomy is a transmission as the grammar sees it: its tokens, and every stretch
// of them it recognised. The tokens outside every span are what the grammar could
// not place, which makes the share of them a measure of how much of a transcript
// is phraseology and how much is not -- the first judge of the transcript itself
// rather than of the callsign alone (docs-fr/05-decisions.md, D63).
type Anatomy struct {
	Tokens     []string      `json:"tokens"`
	Spans      []AnatomySpan `json:"spans"`
	Speaker    Speaker       `json:"speaker"`
	Clearances []Clearance   `json:"clearances,omitempty"`
}

// AnatomySpan is one recognised stretch of tokens, [Start, End).
//
// Types:
//   - "callsign": a number read as a flight number;
//   - "value": any other number, with the role its context gave it; the span
//     includes the phrase that introduced it ("flight level one two zero");
//   - "keyword": a role phrase ("descend", "heading") followed by no number;
//   - "letters": a run of NATO letters, as for a registration;
//   - "operator": an airline named by its telephony, Text holding its ICAO code.
type AnatomySpan struct {
	Type   string `json:"type"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Role   Role   `json:"role,omitempty"`
	Digits string `json:"digits,omitempty"`
	Text   string `json:"text,omitempty"`
}

// Anatomy lays a parse out on its tokens. It is a method of the matcher because
// operators are named through the matcher's telephony table; everything else
// comes from the parse. Clearances carry no token range: the grammar finds them
// from the values around them, not from a stretch of words.
func (m *Matcher) Anatomy(r Result) Anatomy {
	toks, _ := tokenSpans(r.Raw)
	if toks == nil {
		toks = []string{}
	}
	// Empty lists rather than null: a reader counting the words outside every
	// span should not have to tell "nothing recognised" from "missing".
	a := Anatomy{Tokens: toks, Spans: []AnatomySpan{}, Speaker: r.Speaker, Clearances: r.Clearances}

	covered := make([]bool, len(toks))
	mark := func(start, end int) {
		for i := start; i < end && i < len(covered); i++ {
			if i >= 0 {
				covered[i] = true
			}
		}
	}
	for _, v := range r.Values {
		t := "value"
		if v.Role == RoleCallsign {
			t = "callsign"
		}
		a.Spans = append(a.Spans, AnatomySpan{Type: t, Start: v.Word, End: v.End, Role: v.Role, Digits: v.Digits, Text: v.Text})
		mark(v.Word, v.End)
	}
	for i, run := range r.letterRuns {
		if i < len(r.Letters) {
			a.Spans = append(a.Spans, AnatomySpan{Type: "letters", Start: run[0], End: run[1], Text: r.Letters[i]})
			mark(run[0], run[1])
		}
	}
	for code, ranges := range m.operatorsIn(r.Raw) {
		for _, w := range ranges {
			a.Spans = append(a.Spans, AnatomySpan{Type: "operator", Start: w[0], End: w[1], Text: code})
			mark(w[0], w[1])
		}
	}
	// A role phrase the parse did not turn into a value -- "descend" with no
	// number after it -- is still phraseology, not noise.
	for i := 0; i < len(toks); {
		_, role, n := matchKeyword(toks, i)
		if n == 0 {
			i++
			continue
		}
		free := true
		for j := i; j < i+n; j++ {
			if covered[j] {
				free = false
			}
		}
		if free {
			a.Spans = append(a.Spans, AnatomySpan{Type: "keyword", Start: i, End: i + n, Role: role})
			mark(i, i+n)
		}
		i += n
	}

	sort.SliceStable(a.Spans, func(i, j int) bool {
		if a.Spans[i].Start != a.Spans[j].Start {
			return a.Spans[i].Start < a.Spans[j].Start
		}
		return a.Spans[i].End < a.Spans[j].End
	})
	return a
}
