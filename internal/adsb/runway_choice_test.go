package adsb

import (
	"math"
	"testing"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/pkg/logger"
)

type threshold = struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Paris-CDG's four runways, from assets/runways.csv. In each pair the thresholds
// are staggered: 09R lies 0.48 NM further west than 09L, 383 m to its south.
func cdgRunways() RunwayData {
	return RunwayData{Airport: "LFPG", RunwayThresholds: map[string]map[string]threshold{
		"08L-26R": {"08L": {48.99570083618164, 2.5527400970458984}, "26R": {48.99879837036133, 2.610179901123047}},
		"08R-26L": {"08R": {48.99290084838867, 2.565659999847412}, "26L": {48.99489974975586, 2.6024301052093506}},
		"09L-27R": {"09L": {49.02470016479492, 2.5248899459838867}, "27R": {49.02669906616211, 2.561690092086792}},
		"09R-27L": {"09R": {49.020599365234375, 2.5130600929260254}, "27L": {49.02370071411133, 2.5702900886535645}},
	}}
}

func orlyRunways() RunwayData {
	return RunwayData{Airport: "LFPO", RunwayThresholds: map[string]map[string]threshold{
		"06-24": {"06": {48.720001220703125, 2.316920042037964}, "24": {48.73550033569336, 2.360680103302002}},
		"07-25": {"07": {48.719398498535156, 2.3585898876190186}, "25": {48.72740173339844, 2.4020700454711914}},
	}}
}

func choiceConfig() config.FlightPhasesConfig {
	return config.FlightPhasesConfig{
		ApproachMaxDistanceNM:         10,
		ApproachCenterlineToleranceNM: 0.5,
		ApproachHeadingToleranceDeg:   30,
	}
}

// pointOnAxis is d NM from (lat, lon) along bearing (degrees true), flat earth.
func pointOnAxis(lat, lon, bearing, d float64) (float64, float64) {
	b := bearing * math.Pi / 180
	return lat + d*math.Cos(b)/60, lon + d*math.Sin(b)/(60*math.Cos(lat*math.Pi/180))
}

func TestAnApproachGoesToTheNearestCentrelineNotTheNearestThreshold(t *testing.T) {
	rw := cdgRunways()
	for _, c := range []struct{ end, pair, opposite string }{
		{"09L", "09L-27R", "27R"},
		{"09R", "09R-27L", "27L"},
		{"08L", "08L-26R", "26R"},
		{"08R", "08R-26L", "26L"},
	} {
		th := rw.RunwayThresholds[c.pair][c.end]
		op := rw.RunwayThresholds[c.pair][c.opposite]
		heading := CalculateBearing(th.Latitude, th.Longitude, op.Latitude, op.Longitude)
		for _, d := range []float64{3, 6, 9} {
			lat, lon := pointOnAxis(th.Latitude, th.Longitude, heading+180, d)
			got := DetectRunwayApproach(lat, lon, heading, 2500, rw, choiceConfig())
			if got == nil || got.RunwayID != c.pair+"/"+c.end {
				t.Errorf("%s final at %.0f NM: got %+v, want %s/%s", c.end, d, got, c.pair, c.end)
			}
		}
	}
}

func TestAStaggeredParallelCompetesFromTheStartOfTheFinal(t *testing.T) {
	rw := cdgRunways()
	th := rw.RunwayThresholds["09L-27R"]["09L"]
	op := rw.RunwayThresholds["09L-27R"]["27R"]
	heading := CalculateBearing(th.Latitude, th.Longitude, op.Latitude, op.Longitude)
	// 10.2 NM out on the 09L centreline: the 09L threshold is past 10 NM, the
	// 09R one, 0.48 NM nearer, is not.
	lat, lon := pointOnAxis(th.Latitude, th.Longitude, heading+180, 10.2)
	if got := DetectRunwayApproach(lat, lon, heading, 3500, rw, choiceConfig()); got == nil || got.RunwayID != "09L-27R/09L" {
		t.Errorf("start of the 09L final: got %+v, want 09L-27R/09L", got)
	}
	// Past 10 NM from every threshold, no approach at all.
	lat, lon = pointOnAxis(th.Latitude, th.Longitude, heading+180, 10.8)
	if got := DetectRunwayApproach(lat, lon, heading, 3500, rw, choiceConfig()); got != nil {
		t.Errorf("10.8 NM out: got %+v, want no approach", got)
	}
}

func TestADepartureGoesToTheNearestCentreline(t *testing.T) {
	rw := cdgRunways()
	// The receiver, west of CDG: departing aircraft fly away from it.
	stationLat, stationLon := 48.80586, 2.04932
	for _, c := range []struct{ end, pair, opposite string }{
		{"09L", "09L-27R", "27R"},
		{"09R", "09R-27L", "27L"},
	} {
		th := rw.RunwayThresholds[c.pair][c.end]
		op := rw.RunwayThresholds[c.pair][c.opposite]
		heading := CalculateBearing(th.Latitude, th.Longitude, op.Latitude, op.Longitude)
		lat, lon := pointOnAxis(op.Latitude, op.Longitude, heading, 2)
		got := DetectRunwayDeparture(lat, lon, heading, rw, stationLat, stationLon, choiceConfig())
		if got == nil || got.RunwayID != c.pair+"/"+c.end {
			t.Errorf("departure from %s: got %+v, want %s/%s", c.end, got, c.pair, c.end)
		}
	}
}

func newChoiceTracker(t *testing.T, rw RunwayData) *RunwayInUseTracker {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	rt := NewRunwayInUseTracker(60, 2, 3, 2, 0.98, log)
	rt.SetRunwayData(rw)
	return rt
}

func TestParallelRunwaysWithDifferentNumbersAreActiveTogether(t *testing.T) {
	rt := newChoiceTracker(t, cdgRunways())
	for i := 0; i < 5; i++ {
		rt.RecordEvent("09L-27R/09L", RunwayEventApproach, "abc123")
	}
	for _, id := range []string{"09L-27R/09L", "09R-27L/09R", "08L-26R/08L", "08R-26L/08R"} {
		if !rt.IsActiveRunway(id) {
			t.Errorf("%s should be active alongside 09L", id)
		}
	}
	for _, id := range []string{"09L-27R/27R", "08R-26L/26L"} {
		if rt.IsActiveRunway(id) {
			t.Errorf("%s, the opposite direction, should not be active", id)
		}
	}
}

func TestRunwaysTwelveDegreesApartStaySeparate(t *testing.T) {
	rt := newChoiceTracker(t, orlyRunways())
	for i := 0; i < 5; i++ {
		rt.RecordEvent("07-25/25", RunwayEventApproach, "abc123")
	}
	if rt.IsActiveRunway("06-24/24") {
		t.Error("Orly 24 is 12 degrees from 25: not a parallel")
	}
}
