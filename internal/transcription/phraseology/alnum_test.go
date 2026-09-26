package phraseology

import (
	"reflect"
	"testing"
)

func TestSpokenFlightPartsJoinDigitsAndLetters(t *testing.T) {
	cases := map[string][]string{
		"vueling seven uniform echo":              {"7UE"},
		"speed 200 welling seven uniform make on": {"7U"},
		"air france one one november quebec":      {"11NQ"},
		"easy three six victor juliett":           {"36VJ"},
		"descend flight level one two zero":       nil, // a number with no letters is the digit rules' business
	}
	for text, want := range cases {
		var got []string
		for _, g := range alnumGroups(tokenize(text)) {
			got = append(got, g.text)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", text, got, want)
		}
	}
}

func TestSoundsLikeKeepsTheConsonants(t *testing.T) {
	if !soundsLike("welling", "vueling") {
		t.Error("welling should sound like vueling")
	}
	for _, pair := range [][2]string{{"level", "vueling"}, {"speed", "speedbird"}, {"air", "iberia"}} {
		if soundsLike(pair[0], pair[1]) {
			t.Errorf("%q should not sound like %q", pair[0], pair[1])
		}
	}
}

// The case the owner heard on 26/09 on Orly approach: the aircraft was in the
// sky, and the transcript held the airline, the digit and one of two letters.
func TestVuelingSevenUniformIsFoundWithBothRules(t *testing.T) {
	m := &Matcher{telephony: map[string]string{"vueling": "VLG", "airfrans": "AFR"}, MinDigits: 3}
	fleet := []Aircraft{{Callsign: "VLG7UE"}, {Callsign: "VLG8828"}, {Callsign: "VLG359E"},
		{Callsign: "AFR75SH"}, {Callsign: "TUI7WT"}, {Callsign: "EWG7CK"}}
	r := Parse("speed 200 welling seven uniform make on")

	if _, ok := m.Match(r, fleet); ok {
		t.Fatal("with the rules off, as in production, nothing should be found")
	}
	m.AlnumCallsigns = true
	if _, ok := m.Match(r, fleet); ok {
		t.Error("the flight part alone, a letter short, is not enough")
	}
	m.FuzzyOperators = true
	got, ok := m.Match(r, fleet)
	if !ok || got.Callsign != "VLG7UE" {
		t.Errorf("with the airline heard roughly and 7U, want VLG7UE, got %+v (%v)", got, ok)
	}
}

func TestAWholeFlightPartNeedsNoCorroboration(t *testing.T) {
	m := &Matcher{telephony: map[string]string{}, MinDigits: 3, AlnumCallsigns: true}
	fleet := []Aircraft{{Callsign: "AFR11NQ"}, {Callsign: "AFR11NE"}, {Callsign: "EZY36VJ"}}
	got, ok := m.Match(Parse("one one november quebec descend four thousand feet"), fleet)
	if !ok || got.Callsign != "AFR11NQ" {
		t.Errorf("want AFR11NQ, got %+v (%v)", got, ok)
	}
}

// Transcribed on 26/09 at 13:37 on Orly approach, Vueling 7UE being in the sky.
func TestAHeadingBeforeTheCallsignDoesNotHideIt(t *testing.T) {
	text := "lifes are heading zero seven zero five thousand feet QNH one zero two two holding seven uniform echo"
	var groups []string
	for _, g := range alnumGroups(tokenize(text)) {
		groups = append(groups, g.text)
	}
	if !reflect.DeepEqual(groups, []string{"7UE"}) {
		t.Errorf("groups = %v, want [7UE]", groups)
	}
	m := &Matcher{telephony: map[string]string{}, MinDigits: 3, AlnumCallsigns: true}
	got, ok := m.Match(Parse(text), []Aircraft{{Callsign: "VLG7UE", AltitudeFt: 5000}, {Callsign: "AFR75SH"}})
	if !ok || got.Callsign != "VLG7UE" {
		t.Errorf("want VLG7UE, got %+v (%v)", got, ok)
	}
}

func TestWithRulesLeavesTheOriginalAlone(t *testing.T) {
	m := &Matcher{telephony: map[string]string{}, MinDigits: 3}
	c := m.WithRules(Rules{Letters: true, ApproxOperators: true, MinDigits: 4, OneDigitOff: true})
	if !c.AlnumCallsigns || !c.FuzzyOperators || !c.FuzzyDigits || c.MinDigits != 4 {
		t.Errorf("copy: %+v", c)
	}
	if m.AlnumCallsigns || m.FuzzyOperators || m.FuzzyDigits || m.MinDigits != 3 {
		t.Errorf("the original matcher must not change: %+v", m)
	}
}
