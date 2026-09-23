package phraseology

import (
	"reflect"
	"strings"
	"testing"
)

// valuesOf lists what the grammar extracted, as role:text.
func valuesOf(r Result) []string {
	var out []string
	for _, v := range r.Values {
		out = append(out, string(v.Role)+":"+v.Text)
	}
	return out
}

func has(vals []string, want string) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}

// The French model writes accents; the tables were written to match "zéro" and
// "zero" alike. Split on the accent, "zéro" was "z" + "ro" and matched neither.
func TestAccentedFrenchDigitsAreRead(t *testing.T) {
	m := newTestMatcher(t)
	r := Parse("Air France deux six zéro, bonjour")
	got, ok := m.Match(r, []Aircraft{{Callsign: "AFR260", Hex: "39a1b2"}})
	if !ok || got.Callsign != "AFR260" {
		t.Fatalf("got %+v, ok=%v; want AFR260 from \"deux six zéro\"", got, ok)
	}
}

// "degrés" introduces a heading; unread, "300 degrés" was a flight number.
func TestDegreesMakeAHeading(t *testing.T) {
	vals := valuesOf(Parse("Left, 300 degrés, GALS, 25"))
	if !has(vals, "heading:300") {
		t.Errorf("values %v, want heading:300", vals)
	}
}

// "à" is a word, not a gap: "niveau 115 à 190" is FL115 then 190, not 115190.
func TestPrepositionSeparatesNumbers(t *testing.T) {
	vals := valuesOf(Parse("France 926, passez le niveau 115 à 190, retenez."))
	if !has(vals, "flight_level:FL115") {
		t.Errorf("values %v, want flight_level:FL115", vals)
	}
	for _, v := range vals {
		if strings.Contains(v, "115190") {
			t.Errorf("the two numbers were glued: %v", vals)
		}
	}
}

// An ordinal ends a number rather than joining it.
func TestOrdinalIsNotPartOfANumber(t *testing.T) {
	vals := valuesOf(Parse("Avouez, 300, 3ème, villes"))
	if !reflect.DeepEqual(vals, []string{"callsign:300"}) {
		t.Errorf("values %v, want only callsign:300", vals)
	}
	for _, w := range []string{"3eme", "1ere", "1er", "2e", "2nd", "3rd", "4th"} {
		if !isOrdinal(w) {
			t.Errorf("%q should read as an ordinal", w)
		}
	}
	for _, w := range []string{"27l", "300", "eme", "3x"} {
		if isOrdinal(w) {
			t.Errorf("%q is not an ordinal", w)
		}
	}
}

// A runway written as one word keeps its side: the ordinal rule must not take it.
// (Lowercase, as before this change: tokens are lowercased before being read.)
func TestRunwayWithSideStillReads(t *testing.T) {
	vals := valuesOf(Parse("cleared to land runway 27L"))
	if !has(vals, "runway:27l") {
		t.Errorf("values %v, want runway:27l", vals)
	}
}

// NATO letters as the French model spells them.
func TestFrenchSpellingsOfNatoLetters(t *testing.T) {
	for text, want := range map[string]string{
		"C'est toi, Victor Hôtel, exact": "VH",
		"Whisky Québec, bonne journée":   "WQ",
		"heure 24, écho papa":            "EP",
	} {
		if got := Parse(text).Letters; !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("%q: letters %v, want %s", text, got, want)
		}
	}
}

// Matching folds the accents; the text a person reads keeps them.
func TestDisplayKeepsTheAccents(t *testing.T) {
	r := Parse("Barey Contrôle, bonjour, à France 926, passez le niveau 115 à 190")
	want := "barey contrôle bonjour à france 926 passez le niveau FL115 à 190"
	if r.Normalized != want {
		t.Errorf("normalised to %q, want %q", r.Normalized, want)
	}
}

// And the words that named an aircraft are found in that accented text.
func TestEvidenceInAnAccentedDisplay(t *testing.T) {
	m := newTestMatcher(t)
	r := Parse("Réponse, Air France deux six zéro, fréquence")
	got, ok := m.Match(r, []Aircraft{{Callsign: "AFR260", Hex: "39a1b2"}})
	if !ok {
		t.Fatal("no match")
	}
	words := cut(r.Normalized, r.NormalizedSpans(got.Words))
	if want := []string{"air france", "deux six zéro"}; !reflect.DeepEqual(words, want) {
		t.Errorf("highlighted %q in %q, want %q", words, r.Normalized, want)
	}
}
