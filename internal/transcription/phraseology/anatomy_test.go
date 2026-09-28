package phraseology

import "testing"

func spanOf(a Anatomy, typ string) (AnatomySpan, bool) {
	for _, s := range a.Spans {
		if s.Type == typ {
			return s, true
		}
	}
	return AnatomySpan{}, false
}

// Every recognised stretch comes back on the tokens, so what the grammar could
// not place can be counted. The grammar's keywords are the phrases that give a
// number its role; a verb such as "descend" is not one of them, and is left, with
// the noise, to whoever counts with a phraseology lexicon.
func TestAnatomyPlacesEveryRecognisedStretch(t *testing.T) {
	m := &Matcher{telephony: map[string]string{"airfrance": "AFR"}, MinDigits: 3}
	a := m.Anatomy(Parse("Air France one two three blah blah descend flight level one two zero"))

	want := []string{"air", "france", "one", "two", "three", "blah", "blah", "descend", "flight", "level", "one", "two", "zero"}
	if len(a.Tokens) != len(want) {
		t.Fatalf("tokens %v, want %v", a.Tokens, want)
	}
	op, ok := spanOf(a, "operator")
	if !ok || op.Start != 0 || op.End != 2 || op.Text != "AFR" {
		t.Errorf("operator span %+v", op)
	}
	cs, ok := spanOf(a, "callsign")
	if !ok || cs.Start != 2 || cs.End != 5 || cs.Digits != "123" {
		t.Errorf("callsign span %+v", cs)
	}
	var fl AnatomySpan
	for _, s := range a.Spans {
		if s.Type == "value" && s.Role == RoleFlightLevel {
			fl = s
		}
	}
	if fl.Digits != "120" || fl.End != 13 {
		t.Errorf("flight level span %+v", fl)
	}

	covered := make([]bool, len(a.Tokens))
	for _, s := range a.Spans {
		for i := s.Start; i < s.End; i++ {
			covered[i] = true
		}
	}
	var loose []string
	for i, c := range covered {
		if !c {
			loose = append(loose, a.Tokens[i])
		}
	}
	if len(loose) != 3 || loose[0] != "blah" || loose[1] != "blah" || loose[2] != "descend" {
		t.Errorf("unplaced tokens %v, want [blah blah descend]", loose)
	}
}

// A role phrase with no number after it is still placed, as a keyword.
func TestAnatomyPlacesALoneKeyword(t *testing.T) {
	m := &Matcher{telephony: map[string]string{}, MinDigits: 3}
	a := m.Anatomy(Parse("contact approach bonjour"))
	k, ok := spanOf(a, "keyword")
	if !ok || k.Start != 0 || k.End != 1 {
		t.Errorf("keyword span %+v in %+v", k, a.Spans)
	}
}

// Registration letters are a stretch of their own.
func TestAnatomyPlacesLetters(t *testing.T) {
	m := &Matcher{telephony: map[string]string{}, MinDigits: 3}
	a := m.Anatomy(Parse("fox kilo papa victor runway two eight"))
	l, ok := spanOf(a, "letters")
	if !ok || l.Start != 0 || l.End != 4 || l.Text != "FKPV" {
		t.Errorf("letters span %+v in %+v", l, a.Spans)
	}
}
