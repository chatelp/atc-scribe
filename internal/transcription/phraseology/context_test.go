package phraseology

import "testing"

func testMatcher() *Matcher {
	return &Matcher{telephony: map[string]string{}, MinDigits: 3}
}

// Two aircraft whose callsigns both end in the spoken digits are a genuine tie,
// and the matcher refuses ties. Knowing which of the two the frequency was just
// talking to resolves it -- and only that, since both were already named.
func TestContextResolvesATie(t *testing.T) {
	m := testMatcher()
	fleet := []Aircraft{
		{Callsign: "AFR350", Hex: "aaa"},
		{Callsign: "BAW350", Hex: "bbb"},
	}
	r := Parse("three five zero descend flight level one zero zero")

	blind, ok := m.Match(r, fleet)
	if !ok {
		t.Fatal("no match at all, the fixture is wrong")
	}
	if !blind.Ambiguous {
		t.Fatalf("expected an ambiguous match without context, got %q", blind.Callsign)
	}

	withCtx, ok := m.MatchWithContext(r, fleet, []string{"BAW350"})
	if !ok {
		t.Fatal("no match with context")
	}
	if withCtx.Ambiguous {
		t.Error("context should have resolved the tie")
	}
	if withCtx.Callsign != "BAW350" {
		t.Errorf("callsign = %q, want BAW350 -- the one just heard", withCtx.Callsign)
	}
}

// Context may only choose between candidates the transmission already names. An
// aircraft heard a moment ago but not named here must stay unmatched, because the
// shuffled control cannot tell such a match from a correct one.
func TestContextNeverInventsAMatch(t *testing.T) {
	m := testMatcher()
	fleet := []Aircraft{{Callsign: "AFR1081", Hex: "aaa"}}

	r := Parse("descend flight level one zero zero")
	if got, ok := m.MatchWithContext(r, fleet, []string{"AFR1081"}); ok {
		t.Errorf("matched %q from context alone; nothing in the text names it", got.Callsign)
	}
}

// A number the transmission states three times is stronger evidence than one
// heard once.
func TestRepeatedDigitsScoreHigher(t *testing.T) {
	m := testMatcher()
	fleet := []Aircraft{{Callsign: "AFR367", Hex: "aaa"}}

	once, ok := m.Match(Parse("three six seven descend"), fleet)
	if !ok {
		t.Fatal("no match on the single mention")
	}
	thrice, ok := m.Match(Parse("three six seven descend three six seven roger three six seven"), fleet)
	if !ok {
		t.Fatal("no match on the repeated mention")
	}
	if thrice.Score <= once.Score {
		t.Errorf("repeated = %.2f, single = %.2f: repetition bought nothing", thrice.Score, once.Score)
	}
}
