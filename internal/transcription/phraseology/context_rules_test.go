package phraseology

import "testing"

func contextMatcher() *Matcher {
	return &Matcher{telephony: map[string]string{"airfrans": "AFR", "airfrance": "AFR", "lufthansa": "DLH"}, MinDigits: 3}
}

var busySky = []Aircraft{{Callsign: "AFR23SB"}, {Callsign: "AFR11NQ"}, {Callsign: "LBT554"}, {Callsign: "DLH36E"}, {Callsign: "EZY36VJ"}}

// "Sierra Bravo" alone, two minutes after AFR23SB was heard on the frequency.
func TestTheLastLettersOfAnAircraftJustHeard(t *testing.T) {
	m := contextMatcher()
	r := Parse("sierra bravo descend four thousand feet")
	if _, ok := m.MatchWithContext(r, busySky, []string{"AFR23SB"}); ok {
		t.Fatal("off by default, as in production")
	}
	m.ContextLetters = true
	got, ok := m.MatchWithContext(r, busySky, []string{"AFR23SB"})
	if !ok || got.Callsign != "AFR23SB" {
		t.Errorf("want AFR23SB, got %+v (%v)", got, ok)
	}
	if _, ok := m.MatchWithContext(r, busySky, []string{"DLH36E"}); ok {
		t.Error("letters that end no aircraft just heard prove nothing")
	}
}

// "five four" for LBT554, and only when no other aircraft just heard ends so.
func TestTheLastTwoDigitsOfAnAircraftJustHeard(t *testing.T) {
	m := contextMatcher()
	m.ContextDigits = true
	r := Parse("five four turn left heading two seven zero")
	if got, ok := m.MatchWithContext(r, busySky, []string{"LBT554"}); !ok || got.Callsign != "LBT554" {
		t.Errorf("want LBT554, got %+v (%v)", got, ok)
	}
	twin := append([]Aircraft{{Callsign: "BAW254"}}, busySky...)
	if got, ok := m.MatchWithContext(r, twin, []string{"LBT554", "BAW254"}); ok {
		t.Errorf("two aircraft just heard end in 54: nothing may be chosen, got %+v", got)
	}
}

// "Lufthansa" alone, when one Lufthansa was heard on the frequency.
func TestTheAirlineAloneWhenOnlyOneOfItWasJustHeard(t *testing.T) {
	m := contextMatcher()
	m.ContextNames = true
	r := Parse("lufthansa contact one two five eight two five")
	if got, ok := m.MatchWithContext(r, busySky, []string{"DLH36E"}); !ok || got.Callsign != "DLH36E" {
		t.Errorf("want DLH36E, got %+v (%v)", got, ok)
	}
	r = Parse("air france reduce speed one eight zero")
	if got, ok := m.MatchWithContext(r, busySky, []string{"AFR23SB", "AFR11NQ"}); ok {
		t.Errorf("two Air France just heard: nothing may be chosen, got %+v", got)
	}
}
