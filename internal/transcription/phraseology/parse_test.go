package phraseology

import "testing"

// Every input below is real output from a local model on this station's audio,
// copied from docs-fr/. Testing against invented sentences would prove nothing:
// the point is to survive what the models actually emit, noise included.

func find(r Result, role Role) (Value, bool) {
	for _, v := range r.Values {
		if v.Role == role {
			return v, true
		}
	}
	return Value{}, false
}

func TestFlightLevel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"descend flight level three five zero easy nine seven eight six", "FL350"},
		{"departure on golf csa eight charlie whiskey flight level four seven zero on track abdo", "FL470"},
		{"praha approach romeo five six mike xray flight level three eight zero to tepac", "FL380"},
		{"climb level nine zero holding one two three", "FL90"},
	}
	for _, c := range cases {
		v, ok := find(Parse(c.in), RoleFlightLevel)
		if !ok || v.Text != c.want {
			t.Errorf("%q: got %q (found=%v), want %q", c.in, v.Text, ok, c.want)
		}
	}
}

func TestRunwayAndSide(t *testing.T) {
	cases := []struct{ in, want string }{
		{"cleared to land runway two five delta foxtrot delta oscar kilo yankee", "25"},
		{"and good bye sir cleared to land runway two seven air malik", "27"},
		{"cleared for takeoff runway zero eight left", "08L"},
		{"tower alfa four two eight romeo can we vacate via runway", ""},
	}
	for _, c := range cases {
		v, ok := find(Parse(c.in), RoleRunway)
		if c.want == "" {
			if ok {
				t.Errorf("%q: found runway %q, expected none", c.in, v.Text)
			}
			continue
		}
		if !ok || v.Text != c.want {
			t.Errorf("%q: got %q (found=%v), want %q", c.in, v.Text, ok, c.want)
		}
	}
}

func TestFrequencyNotMistakenForCallsign(t *testing.T) {
	// This one cost a false match in the first naive extractor: a frequency
	// handoff read as an aircraft.
	r := Parse("you just kind of lost some of your background there contact one two one decimal zero five five thank you very much")
	v, ok := find(r, RoleFrequency)
	if !ok || v.Text != "121.055" {
		t.Fatalf("got %q (found=%v), want 121.055", v.Text, ok)
	}
	for _, x := range r.Values {
		if x.Role == RoleCallsign && x.Digits == "121" {
			t.Errorf("frequency 121 leaked into callsign candidates")
		}
	}
}

func TestSpeed(t *testing.T) {
	v, ok := find(Parse("speed one five zero knots jet two two nine"), RoleSpeed)
	if !ok || v.Digits != "150" {
		t.Errorf("got %q (found=%v), want 150", v.Digits, ok)
	}
}

func TestHeadingIsPaddedToThreeDigits(t *testing.T) {
	v, ok := find(Parse("air france eight zero two foxtrot proceed with descend heading zero zero"), RoleHeading)
	if !ok || v.Text != "000" {
		t.Errorf("got %q (found=%v), want 000", v.Text, ok)
	}
}

func TestCallsignCandidates(t *testing.T) {
	// A bare digit group with no role keyword is the flight number.
	r := Parse("air france one zero eight one good good day")
	var got []string
	for _, v := range r.Values {
		if v.Role == RoleCallsign {
			got = append(got, v.Digits)
		}
	}
	if len(got) != 1 || got[0] != "1081" {
		t.Errorf("got %v, want [1081]", got)
	}
}

func TestLightAircraftLetters(t *testing.T) {
	// French club traffic is spelled out letter by letter, never numbered.
	r := Parse("Fox Alpha Charlie 28 au tourisme d'atterrissage on atterrit piste 28 Fox Alpha Charlie")
	if len(r.Letters) == 0 || r.Letters[0] != "FAC" {
		t.Errorf("got %v, want first group FAC", r.Letters)
	}
}

func TestClearanceExtraction(t *testing.T) {
	r := Parse("delta oscar kilo yankee cleared to land runway two five")
	if len(r.Clearances) != 1 {
		t.Fatalf("got %d clearances, want 1", len(r.Clearances))
	}
	if r.Clearances[0].Type != "landing" || r.Clearances[0].Runway != "25" {
		t.Errorf("got %+v, want landing on 25", r.Clearances[0])
	}
}

func TestSpeakerDetection(t *testing.T) {
	cases := []struct {
		in   string
		want Speaker
	}{
		{"cleared to land runway two seven", SpeakerATC},
		{"hold short for papa alfa five six one", SpeakerATC},
		{"tower alfa four two eight romeo request descent", SpeakerPilot},
		{"front control india one five niner good day with you level is three eight zero", SpeakerPilot},
	}
	for _, c := range cases {
		if got := Parse(c.in).Speaker; got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeWritesDigits(t *testing.T) {
	// Upstream asks a language model to do exactly this rewrite.
	got := Parse("descend flight level three five zero").Normalized
	want := "descend flight level FL350"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGarbageDoesNotPanic(t *testing.T) {
	for _, s := range []string{"", "   ", "opharnentureth", "«Jepenst er brent, ja»",
		"7 and 7 and 7 and 7 and 7", "Я mandателем лейтенка"} {
		_ = Parse(s)
	}
}
