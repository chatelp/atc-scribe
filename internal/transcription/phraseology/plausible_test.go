package phraseology

import "testing"

func valuesByRole(r Result) map[Role][]string {
	out := map[Role][]string{}
	for _, v := range r.Values {
		out[v.Role] = append(out[v.Role], v.Digits)
	}
	return out
}

// A role word after the number let the number swallow the callsign spoken just
// before it: a speed of 1405180 (27/09). The value keeps its plausible end, and
// the callsign comes back as a callsign.
func TestATrailingRoleDoesNotSwallowTheCallsign(t *testing.T) {
	got := valuesByRole(Parse("KLM One Four Zero Five one eighty knots"))
	if len(got[RoleSpeed]) != 1 || got[RoleSpeed][0] != "180" {
		t.Errorf("speed %v, want [180]", got[RoleSpeed])
	}
	if len(got[RoleCallsign]) != 1 || got[RoleCallsign][0] != "1405" {
		t.Errorf("callsign %v, want [1405]", got[RoleCallsign])
	}
}

// The same with the role word in front: the value keeps its plausible start.
func TestALeadingRoleStopsAtAPlausibleValue(t *testing.T) {
	got := valuesByRole(Parse("QNH one zero one three one four zero five"))
	if len(got[RoleQNH]) != 1 || got[RoleQNH][0] != "1013" {
		t.Errorf("qnh %v, want [1013]", got[RoleQNH])
	}
	if len(got[RoleCallsign]) != 1 || got[RoleCallsign][0] != "1405" {
		t.Errorf("callsign %v, want [1405]", got[RoleCallsign])
	}
}

// Plausible values are left exactly as they were read.
func TestPlausibleValuesAreUntouched(t *testing.T) {
	for text, want := range map[string][2]string{
		"descend four thousand feet":       {string(RoleAltitude), "4000"},
		"reduce speed one eight zero":      {string(RoleSpeed), "180"},
		"turn left heading two seven zero": {string(RoleHeading), "270"},
		"squawk seven four two one":        {string(RoleSquawk), "7421"},
	} {
		got := valuesByRole(Parse(text))[Role(want[0])]
		if len(got) != 1 || got[0] != want[1] {
			t.Errorf("%q: %s %v, want [%s]", text, want[0], got, want[1])
		}
	}
}

// A value with no plausible part is a number misheard: it keeps its digits and
// loses the role it cannot have, as an impossible flight level already did.
func TestAnImpossibleValueLosesItsRole(t *testing.T) {
	for _, text := range []string{"QNH three", "speed six knots", "six knots"} {
		for _, v := range Parse(text).Values {
			if v.Role == RoleQNH || v.Role == RoleSpeed {
				t.Errorf("%q: kept %s=%s", text, v.Role, v.Digits)
			}
		}
	}
}
