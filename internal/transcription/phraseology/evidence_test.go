package phraseology

import (
	"reflect"
	"testing"
	"unicode/utf16"
)

// cut returns the pieces of s the spans point at, indexing in UTF-16 code units
// exactly as the browser will.
func cut(s string, spans [][2]int) []string {
	u := utf16.Encode([]rune(s))
	var out []string
	for _, sp := range spans {
		if sp[0] < 0 || sp[1] > len(u) || sp[0] >= sp[1] {
			out = append(out, "<out of range>")
			continue
		}
		out = append(out, string(utf16.Decode(u[sp[0]:sp[1]])))
	}
	return out
}

// The words that named the aircraft, placed in the text a person reads. The
// level is corroboration: it raised the score, it named nobody.
func TestTheWordsThatNamedTheAircraftAreKept(t *testing.T) {
	m := newTestMatcher(t)
	r := Parse("air france one zero eight one descend flight level one two zero")
	got, ok := m.Match(r, []Aircraft{{Callsign: "AFR1081", Hex: "39c1a2", AltitudeFt: 12000}})
	if !ok {
		t.Fatal("no match")
	}
	words := cut(r.Normalized, r.NormalizedSpans(got.Words))
	if want := []string{"air france", "one zero eight one"}; !reflect.DeepEqual(words, want) {
		t.Errorf("highlighted %q in %q, want %q", words, r.Normalized, want)
	}
}

// Normalisation rewrites "flight level three five zero" as "flight level FL350",
// which moves everything after it. The callsign after it must still be found.
func TestEvidenceSurvivesARewrittenValueBeforeIt(t *testing.T) {
	m := newTestMatcher(t)
	r := Parse("flight level three five zero air france one zero eight one")
	got, ok := m.Match(r, []Aircraft{{Callsign: "AFR1081", Hex: "39c1a2"}})
	if !ok {
		t.Fatal("no match")
	}
	if r.Normalized != "flight level FL350 air france one zero eight one" {
		t.Fatalf("precondition: normalised to %q", r.Normalized)
	}
	words := cut(r.Normalized, r.NormalizedSpans(got.Words))
	if want := []string{"air france", "one zero eight one"}; !reflect.DeepEqual(words, want) {
		t.Errorf("highlighted %q, want %q", words, want)
	}
}

// The second reading is displayed as decoded: capitals, punctuation, and a French
// accent before the callsign, which is one UTF-16 unit and two bytes.
func TestEvidenceInTheRawText(t *testing.T) {
	m := newTestMatcher(t)
	raw := "Réponse : Air France, one zero eight one."
	r := Parse(raw)
	got, ok := m.Match(r, []Aircraft{{Callsign: "AFR1081", Hex: "39c1a2"}})
	if !ok {
		t.Fatal("no match")
	}
	words := cut(raw, r.RawSpans(got.Words))
	if want := []string{"Air France", "one zero eight one"}; !reflect.DeepEqual(words, want) {
		t.Errorf("highlighted %q in %q, want %q", words, raw, want)
	}
}

// A callsign said twice counted twice, so both are evidence.
func TestRepeatedDigitsAreEvidenceEachTime(t *testing.T) {
	m := newTestMatcher(t)
	r := Parse("one zero eight one contact de gaulle one zero eight one")
	got, ok := m.Match(r, []Aircraft{{Callsign: "AFR1081", Hex: "39c1a2"}})
	if !ok {
		t.Fatal("no match")
	}
	words := cut(r.Normalized, r.NormalizedSpans(got.Words))
	if want := []string{"one zero eight one", "one zero eight one"}; !reflect.DeepEqual(words, want) {
		t.Errorf("highlighted %q, want both occurrences", words)
	}
}

// Letter tails are read out as NATO words and count towards the match.
func TestLettersAreEvidence(t *testing.T) {
	m := newTestMatcher(t)
	r := Parse("one two three victor juliett descend")
	got, ok := m.Match(r, []Aircraft{{Callsign: "EZY123VJ", Hex: "400b2c"}})
	if !ok {
		t.Fatal("no match")
	}
	words := cut(r.Normalized, r.NormalizedSpans(got.Words))
	if want := []string{"one two three", "victor juliett"}; !reflect.DeepEqual(words, want) {
		t.Errorf("highlighted %q, want digits and letters", words)
	}
}

// Positions are in UTF-16 units and must stay so past characters that are
// several bytes in UTF-8.
func TestTokenSpansCountUTF16Units(t *testing.T) {
	toks, spans := tokenSpans("été, Air France")
	if !reflect.DeepEqual(toks, []string{"t", "air", "france"}) {
		t.Fatalf("tokens %q", toks)
	}
	want := []Span{{1, 2}, {5, 8}, {9, 15}}
	if !reflect.DeepEqual(spans, want) {
		t.Errorf("spans %v, want %v", spans, want)
	}
}
