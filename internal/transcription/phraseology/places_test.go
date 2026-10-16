package phraseology

import "testing"

// Points of Paris procedures, where they are near enough for a test.
var testPlaces = []PlacePoint{{Name: "ELCOB", Lat: 49.03, Lon: 1.22}, {Name: "RANUX", Lat: 49.14, Lon: 4.36},
	{Name: "OKIPA", Lat: 48.61, Lon: 3.56}, {Name: "OPALE", Lat: 49.9, Lon: 1.88}, {Name: "ATREX", Lat: 49.79, Lon: 2.37},
	{Name: "MOPAR", Lat: 49.29, Lon: 1.76}, {Name: "DORDI", Lat: 48.9, Lon: 2.9}}

func placesIn(t *testing.T, n *PlaceNamer, text string, floor float64) []Place {
	t.Helper()
	return n.Name(Parse(text), floor)
}

// The owner's listening (16/10): procedures named, and what must stay unnamed.
func TestTheProceduresHeardAreNamed(t *testing.T) {
	n := NewPlaceNamer(newTestMatcher(t))
	n.Set(testPlaces)
	for _, c := range []struct{ text, want string }{
		{"elcope seven golf climb flight level one two zero", "ELCOB 7G"},
		{"Ranux Seven Golf, descend flight level eight zero", "RANUX 7G"},
		{"cleared Oquipa one romeo arrival", "OKIPA 1R"},
		{"Atrex 7 Golf, contact departure", "ATREX 7G"},
		// A callsign: two letters after the digit.
		{"Anatolia Three Foxtrot Quebec descend", ""},
		// An airline's name before, or as the word: a callsign spoken short.
		{"air france seven golf contact", ""},
		{"Etihad Three Foxtrot Four contact", ""},
		// Phraseology, or a word too short to be sure ("dot" sounds like DORDI).
		{"descend seven golf", ""},
		{"ready to takeoff dot two whiskey zero two", ""},
		// A word that sounds like no procedure point.
		{"cruise one echo", ""},
	} {
		got := placesIn(t, n, c.text, 0.95)
		name := ""
		if len(got) > 0 {
			name = got[0].Designator
		}
		if name != c.want {
			t.Errorf("%q: %q, want %q", c.text, name, c.want)
		}
	}
}

func TestTheFloorIsTheSettingsAndTheWordsAreFound(t *testing.T) {
	n := NewPlaceNamer(newTestMatcher(t))
	n.Set(testPlaces)
	r := Parse("Air France five two, elcope seven golf, climb")
	got := n.Name(r, 0.95)
	if len(got) != 1 || got[0].Point != "ELCOB" || got[0].Lat != 49.03 {
		t.Fatalf("%+v", got)
	}
	if s := r.RawSpans([][2]int{got[0].Words}); len(s) != 1 || r.Raw[s[0][0]:s[0][1]] != "elcope seven golf" {
		t.Errorf("words %v in %q", s, r.Raw)
	}
	// Stricter, the same word is not named: "elcope" scores 1.06.
	if got := n.Name(r, 1.10); len(got) != 0 {
		t.Errorf("at 1.10: %+v", got)
	}
}

func TestNoPointsNoNames(t *testing.T) {
	n := NewPlaceNamer(newTestMatcher(t))
	if got := n.Name(Parse("elcope seven golf"), 0.95); got != nil || n.Count() != 0 {
		t.Errorf("%+v", got)
	}
	n.Set(testPlaces)
	if n.Count() != len(testPlaces) {
		t.Errorf("%d points", n.Count())
	}
}

func TestAPlaceSaysItsKind(t *testing.T) {
	n := NewPlaceNamer(newTestMatcher(t))
	n.Set([]PlacePoint{{Name: "ELCOB", Kinds: map[string]string{"ELCOB 7A": "SID", "ELCOB 7D": "SID"}},
		{Name: "MOPAR", Kinds: map[string]string{"MOPAR 1E": "SID", "MOPAR 2W": "STAR"}}})
	for _, c := range []struct{ text, kind string }{
		{"elcope seven golf", "SID"}, // not in the tables, but every ELCOB procedure is a SID
		{"mopar one echo", "SID"},    // in the tables
		{"mopar three whiskey", ""},  // MOPAR has both kinds: not known
	} {
		got := n.Name(Parse(c.text), 0.95)
		if len(got) != 1 || got[0].Kind != c.kind {
			t.Errorf("%q: %+v, want kind %q", c.text, got, c.kind)
		}
	}
}
