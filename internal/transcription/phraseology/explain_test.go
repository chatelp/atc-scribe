package phraseology

import (
	"reflect"
	"testing"
)

func plainTexts(s string) []string {
	var out []string
	for _, in := range Parse(s).Plain() {
		out = append(out, in.Text)
	}
	return out
}

func TestATransmissionReadsInPlainWords(t *testing.T) {
	for _, c := range []struct {
		said string
		want []string
	}{
		{"air france one two three descend flight level one zero zero", []string{"Descend to FL100"}},
		{"air france one two three climb and maintain flight level two four zero", []string{"Climb to FL240"}},
		{"easy seven seven turn left heading two seven zero", []string{"Turn left heading 270"}},
		{"ryanair four five reduce speed two two zero knots", []string{"Reduce speed to 220 kt"}},
		{"air france one two three contact de gaulle one two six decimal four two five", []string{"Contact 126.425"}},
		{"air france one two three descend four thousand feet QNH one zero one five", []string{"Descend to 4000 ft", "QNH 1015"}},
		{"air france one two three runway two seven right cleared to land", []string{"Cleared to land, runway 27R"}},
		{"split zero one five hundred feet", []string{"1500 ft"}},
		{"lepin france four three one green alpha remontez vers niveau un zero zero", []string{"Climb to FL100"}},
		{"cleared ils one two left cap zero two four", []string{"Heading 024"}},
		{"to the right cap zero two four authorized ils cap zero two four", []string{"Turn right heading 024"}},
		{"bonjour", nil},
	} {
		if got := plainTexts(c.said); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: %q, want %q", c.said, got, c.want)
		}
	}
}
