package phraseology

import "testing"

// The fleet below is a real snapshot of what the station was seeing, trimmed.
var fleet = []Aircraft{
	{Callsign: "ICE546", Hex: "4ccfb1"},
	{Callsign: "MEA229", Hex: "74806d"},
	{Callsign: "DAL229", Hex: "a1b2c3"},
	{Callsign: "AFR1081", Hex: "3949e9"},
	{Callsign: "EZY9786", Hex: "4ca123"},
	{Callsign: "RYR71SH", Hex: "4ca801"},
	{Callsign: "BAW522", Hex: "4063de"},
	{Callsign: "EZY36VJ", Hex: "4cabcd"},
}

func newTestMatcher(t *testing.T) *Matcher {
	t.Helper()
	m, err := NewMatcher("../../../assets/airlines.dat")
	if err != nil {
		t.Skipf("airlines.dat unavailable: %v", err)
	}
	return m
}

func TestMatchOnDigitsAlone(t *testing.T) {
	// No operator named, and the surrounding words are nonsense — this is real
	// output. The digits still carry it.
	m := newTestMatcher(t)
	got, ok := m.Match(Parse("to the right cleared to land i say five four six"), fleet)
	if !ok || got.Callsign != "ICE546" {
		t.Fatalf("got %+v, ok=%v; want ICE546", got, ok)
	}
}

func TestOperatorNameRaisesScore(t *testing.T) {
	m := newTestMatcher(t)
	with, _ := m.Match(Parse("air france one zero eight one good day"), fleet)
	without, _ := m.Match(Parse("one zero eight one good day"), fleet)
	if with.Callsign != "AFR1081" || without.Callsign != "AFR1081" {
		t.Fatalf("both should reach AFR1081: %q / %q", with.Callsign, without.Callsign)
	}
	if with.Score <= without.Score {
		t.Errorf("naming the operator should raise the score: %v vs %v", with.Score, without.Score)
	}
}

func TestSpokenTelephonyName(t *testing.T) {
	// Controllers say "Speedbird", not "British Airways". airlines.dat holds both.
	m := newTestMatcher(t)
	got, ok := m.Match(Parse("speedbird five two two descend flight level one two zero"), fleet)
	if !ok || got.Callsign != "BAW522" {
		t.Fatalf("got %+v, ok=%v; want BAW522", got, ok)
	}
}

func TestAmbiguityIsReportedNotGuessed(t *testing.T) {
	// 229 is both MEA229 and DAL229 and nothing in the words separates them.
	m := newTestMatcher(t)
	got, ok := m.Match(Parse("speed one five zero knots jet two two nine"), fleet)
	if !ok {
		t.Fatal("expected a match")
	}
	if !got.Ambiguous || len(got.Runners) == 0 {
		t.Errorf("expected ambiguity between MEA229 and DAL229, got %+v", got)
	}
}

func TestHallucinatedOperatorDoesNotBlockTheMatch(t *testing.T) {
	// Air Berlin stopped flying in 2017; the model says it anyway. Naming a
	// carrier that is not there must cost nothing, because the digits are sound.
	m := newTestMatcher(t)
	got, ok := m.Match(Parse("right air berlin five four six confirm direct"), fleet)
	if !ok || got.Callsign != "ICE546" {
		t.Fatalf("got %+v, ok=%v; want ICE546 regardless of the invented operator", got, ok)
	}
}

func TestFrequencyIsNotAnAircraft(t *testing.T) {
	// The naive extractor matched 121 and 1220 against the fleet. It must not.
	m := newTestMatcher(t)
	if got, ok := m.Match(Parse("contact one two one decimal zero five five good bye"), fleet); ok {
		t.Errorf("a frequency handoff matched %q", got.Callsign)
	}
}

func TestNoFleetNoMatch(t *testing.T) {
	m := newTestMatcher(t)
	if _, ok := m.Match(Parse("air france one zero eight one"), nil); ok {
		t.Error("matched against an empty sky")
	}
}

func TestSplitCallsign(t *testing.T) {
	cases := []struct{ in, p, d, l string }{
		{"AFR1081", "AFR", "1081", ""},
		{"EZY36VJ", "EZY", "36", "VJ"},
		{"RYR71SH", "RYR", "71", "SH"},
		{"FHARI", "FHARI", "", ""},
	}
	for _, c := range cases {
		p, d, l := splitCallsign(c.in)
		if p != c.p || d != c.d || l != c.l {
			t.Errorf("%s: got (%q,%q,%q), want (%q,%q,%q)", c.in, p, d, l, c.p, c.d, c.l)
		}
	}
}
