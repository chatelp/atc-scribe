package phraseology

import (
	"reflect"
	"testing"
)

func plainTextsFR(s string) []string {
	var out []string
	for _, in := range Parse(s).Plain() {
		out = append(out, in.TextFR)
	}
	return out
}

// Each instruction carries its plain French too (13/10, D77): a template per
// role, "niveau" for a flight level, the values as they were said.
func TestATransmissionReadsInPlainFrenchToo(t *testing.T) {
	for _, c := range []struct {
		said string
		want []string
	}{
		{"air france one two three descend flight level one zero zero", []string{"Descendez au niveau 100"}},
		{"air france one two three climb and maintain flight level two four zero", []string{"Montez au niveau 240"}},
		{"easy seven seven turn left heading two seven zero", []string{"Tournez à gauche au cap 270"}},
		{"ryanair four five reduce speed two two zero knots", []string{"Réduisez la vitesse à 220 kt"}},
		{"air france one two three contact de gaulle one two six decimal four two five", []string{"Contactez 126.425"}},
		{"air france one two three descend four thousand feet QNH one zero one five", []string{"Descendez au 4000 ft", "QNH 1015"}},
		{"air france one two three runway two seven right cleared to land", []string{"Autorisé à atterrir, piste 27R"}},
		{"cleared ils one two left cap zero two four", []string{"Cap 024"}},
		{"bonjour", nil},
	} {
		if got := plainTextsFR(c.said); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: %q, want %q", c.said, got, c.want)
		}
	}
}

// The French reading says the same value with the same unit: only the words
// change ("niveau 100" is value 100, unit FL).
func TestTheFrenchReadingKeepsTheValueAndUnit(t *testing.T) {
	for _, c := range []struct {
		said, fr, value, unit string
	}{
		{"air france one two three descend flight level one zero zero", "Descendez au niveau 100", "100", "FL"},
		{"lepin france four three one green alpha remontez vers niveau un zero zero", "Montez au niveau 100", "100", "FL"},
		{"easy seven seven turn left heading two seven zero", "Tournez à gauche au cap 270", "270", "deg"},
		{"air france one two three contact de gaulle one two six decimal four two five", "Contactez 126.425", "126.425", "MHz"},
		{"air france one two three runway two seven right cleared to land", "Autorisé à atterrir, piste 27R", "27R", ""},
	} {
		plain := Parse(c.said).Plain()
		if len(plain) != 1 || plain[0].TextFR != c.fr || plain[0].Value != c.value || plain[0].Unit != c.unit {
			t.Errorf("%q: %+v, want %q with %q %q", c.said, plain, c.fr, c.value, c.unit)
		}
	}
}
