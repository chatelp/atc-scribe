package phraseology

import (
	"os"
	"path/filepath"
	"testing"
)

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

// The corrections in telephony-overrides.csv matter through disambiguation: a
// named operator that is in the sky restricts the candidates to its aircraft.
// Each fleet below holds two aircraft with the same digits, so without the
// operator the match is ambiguous or wrong.

func TestCurrentTelephonyDisambiguates(t *testing.T) {
	m := newTestMatcher(t)
	cases := []struct {
		name, said, want, twin string
	}{
		// airlines.dat calls Transavia France "FRENCH SUN".
		{"renamed", "france soleil one two three four", "TVF1234", "AFR1234"},
		// Wizz Air UK is absent from airlines.dat, and was never observed either.
		// (easyJet Europe would not do here: spoken-operators.csv already knew
		// "alpine", so that case passed without the correction and proved nothing.)
		{"absent", "wizz go four five six", "WUK456", "EZY456"},
		// airlines.dat files BEE-LINE under DAT, the code Brussels Airlines retired.
		{"retired code", "bee line three six seven", "BEL367", "AFR367"},
		// FedEx is in airlines.dat but marked inactive, which the matcher skips.
		{"marked inactive", "fedex five one two heavy", "FDX512", "BAW512"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sky := []Aircraft{{Callsign: c.twin, Hex: "aaaaaa"}, {Callsign: c.want, Hex: "bbbbbb"}}
			got, ok := m.Match(Parse(c.said), sky)
			if !ok || got.Callsign != c.want {
				t.Fatalf("%q: got %+v, ok=%v; want %s", c.said, got, ok, c.want)
			}
			if got.Ambiguous {
				t.Errorf("%q: naming the operator should settle it, got ambiguous %+v", c.said, got)
			}
		})
	}
}

func TestWithoutOverridesTheMatcherIsUnchanged(t *testing.T) {
	// The file is optional. Without it, nothing fails and nothing is guessed:
	// "france soleil" simply names no operator, as before the file existed.
	src := "../../../assets/airlines.dat"
	data, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("airlines.dat unavailable: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "airlines.dat"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := NewMatcher(filepath.Join(dir, "airlines.dat"))
	if err != nil {
		t.Fatalf("a missing overrides file must not be an error: %v", err)
	}
	if ops := m.operatorsIn("france soleil one two three four"); len(ops) != 0 {
		t.Errorf("without the file, france soleil should name nothing, got %v", ops)
	}
	if ops := m.operatorsIn("speedbird five two two"); !ops["BAW"] {
		t.Errorf("airlines.dat must still work on its own, got %v", ops)
	}
}

func TestOverridesWinOverAirlinesDat(t *testing.T) {
	// The one real conflict: airlines.dat maps BEE-LINE to DAT. The current code
	// is BEL, and the correction must take the key.
	m := newTestMatcher(t)
	ops := m.operatorsIn("bee line three six seven")
	if !ops["BEL"] || ops["DAT"] {
		t.Errorf("bee line should name BEL and not DAT, got %v", ops)
	}
}
