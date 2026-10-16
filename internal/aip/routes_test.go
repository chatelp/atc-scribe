package aip

import (
	"strings"
	"testing"
)

// A made-up coding table in the layout pdftotext gives the SIA's (no AIP
// data in the repository): a header on three lines, two procedures, a
// value missing without a dash, "FL 070" in two words.
const madeUpTable = `
                                                                  SID RNAV RWY 26L - 26R

    Procedure           Path        Waypoint                  Direction MAG Direction True      Distance                     MNM Altitude    MAX Altitude
                                                 Fly Over                                                Turn direction                                      MAX IAS (kt) Navigation Accuracy (NM)
  Identification     Terminator   Identification                    (°)           (°)            (NM)                       (FL or AMSL ft) (FL or AMSL ft)

    ABCDE 1A
     RWY26L                                                 -            264          265.3
                       CF           ZZ001           -                 -             -             -             -                  -               -                  -                       1.0
     RWY26R                                                 -            264          265.3
        -              TF           ABCDE           -                 200         201.0          12.0           -               FL 070          FL 070                220                     1.0
    FGHIJ 1A
        -              CF           ZZ001           -                 264         265.3           3.0           -                 3000             -                  -                       1.0
        -              TF           FGHIJ           -                 150         151.2          20.1           -                                FL120                250                     1.0
`

func TestACodingTableIsReadByColumn(t *testing.T) {
	var note RoutesNote
	routes := parseCodingTable(madeUpTable, "LFXX", "SID", "AD_2_LFXX_DATA_SID_RWY26L-26R_RNAV_CODE_01", &note)
	if len(routes) != 2 {
		t.Fatalf("routes %+v", routes)
	}
	a, f := routes[0], routes[1]
	if a.Name != "ABCDE 1A" || strings.Join(a.Runways, ",") != "26L,26R" || len(a.Legs) != 2 {
		t.Fatalf("first route %+v", a)
	}
	if l := a.Legs[1]; l.Point != "ABCDE" || l.MinAlt != "FL070" || l.MaxAlt != "FL070" || l.MaxKt != 220 {
		t.Errorf("FL 070 exact and 220 kt: %+v", l)
	}
	if l := f.Legs[0]; l.MinAlt != "3000" || l.MaxAlt != "" || l.MaxKt != 0 {
		t.Errorf("at or above 3000 ft: %+v", l)
	}
	if l := f.Legs[1]; l.MinAlt != "" || l.MaxAlt != "FL120" || l.MaxKt != 250 {
		t.Errorf("an empty cell, then FL120 and 250 kt: %+v", l)
	}
	if note.ConstraintDoubted != 0 || note.Legs != 4 {
		t.Errorf("note %+v", note)
	}

	named := map[string]Point{"ABCDE": {Name: "ABCDE", Lat: 48.5, Lon: 2.0}, "FGHIJ": {Name: "FGHIJ", Lat: 48.2, Lon: 2.3}}
	technical := map[string][2]float64{"ZZ001": {49.0, 2.4}}
	thr := map[string]map[string][2]float64{"LFXX": {"26L": {49.0, 2.6}, "26R": {49.01, 2.61}}}
	placeRoutes(routes, named, nil, technical, thr)
	for _, r := range routes {
		if !r.Complete || len(r.Thresholds) != 2 {
			t.Errorf("%s: complete %v, thresholds %v", r.Name, r.Complete, r.Thresholds)
		}
	}
	if routes[0].Legs[0].Named || !routes[0].Legs[1].Named {
		t.Errorf("a technical point is not named, a reporting point is: %+v", routes[0].Legs)
	}
	delete(technical, "ZZ001")
	placeRoutes(routes, named, nil, technical, thr)
	if routes[0].Complete {
		t.Error("a route with a point the tables do not place is not whole")
	}
}

func TestARouteGivenOncePerRunwayIsOne(t *testing.T) {
	leg := []Leg{{Point: "ABCDE", MinAlt: "FL100"}}
	out := mergeRunways([]Route{
		{Airport: "LFXX", Name: "ABCDE 1W", Kind: "STAR", Runways: []string{"26R"}, Legs: leg},
		{Airport: "LFXX", Name: "ABCDE 1W", Kind: "STAR", Runways: []string{"26L"}, Legs: leg},
	})
	if len(out) != 1 || strings.Join(out[0].Runways, ",") != "26L,26R" {
		t.Fatalf("%+v", out)
	}
}

func TestRunwayThresholdsFromAD212(t *testing.T) {
	page := "<p>AD 2.12 Caractéristiques physiques des pistes</p><p>26L 265.37  (264) 2700 x 60 100 R/B/W/T béton 48°59'41.56\"N 002°36'08.77\"E ------ GUND</p>"
	thr := thresholds(page)
	c, ok := thr["26L"]
	if !ok || c[0] < 48.99 || c[0] > 49.0 || c[1] < 2.60 || c[1] > 2.61 {
		t.Fatalf("thresholds %v", thr)
	}
}
