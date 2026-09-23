package reference

import (
	"sync"
	"testing"

	"github.com/yegors/co-atc/pkg/logger"
)

// Around a public point, central Paris, not any receiver's position. Only the
// airport and runway files are loaded: they are what the home airport needs.
func newParisService(t *testing.T, home string) *Service {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService(ServiceConfig{
		AirportsCSVPath: "../../assets/airports.csv",
		RunwaysCSVPath:  "../../assets/runways.csv",
		StationLat:      48.853, StationLon: 2.349,
		HomeAirportCode: home, DisplayRangeNM: 100, ExtensionLengthNM: 10,
	}, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.airports) == 0 || len(s.runways) == 0 {
		t.Skip("reference data unavailable")
	}
	return s
}

func TestSetHomeAirportReplacesTheRunways(t *testing.T) {
	s := newParisService(t, "LFPZ")
	before := s.GetHomeRunwayData()
	if before.Airport != "LFPZ" {
		t.Fatalf("precondition: home should start as LFPZ, got %q", before.Airport)
	}

	ap, n, err := s.SetHomeAirport("lfpo") // case does not matter
	if err != nil {
		t.Fatalf("Orly should be accepted: %v", err)
	}
	if ap.Ident != "LFPO" || n < 2 {
		t.Fatalf("expected Orly with its runways, got %s with %d", ap.Ident, n)
	}
	after := s.GetHomeRunwayData()
	if after.Airport != "LFPO" || len(after.RunwayThresholds) != n {
		t.Errorf("runway data should now be Orly's %d runways, got %q with %d",
			n, after.Airport, len(after.RunwayThresholds))
	}
	if len(s.GetHomeRunwayExtensions()) != n {
		t.Errorf("extended centrelines should follow the new airport")
	}
	if s.HomeAirportCode() != "LFPO" {
		t.Errorf("home code should be LFPO, got %s", s.HomeAirportCode())
	}
}

func TestSetHomeAirportRefusesRatherThanGuesses(t *testing.T) {
	s := newParisService(t, "LFPZ")

	if _, _, err := s.SetHomeAirport("ZZZZ"); err == nil {
		t.Error("an unknown airport must be refused")
	}
	// Far outside the 100 NM circle: exists in the world, not in the data.
	if _, _, err := s.SetHomeAirport("KJFK"); err == nil {
		t.Error("an airport beyond the display range must be refused")
	}
	// An airport in range none of whose runways has both ends located.
	for code := range s.airportMap {
		if len(s.usableRunways(code)) == 0 {
			if _, _, err := s.SetHomeAirport(code); err == nil {
				t.Errorf("%s has no usable runway and must be refused", code)
			}
			break
		}
	}
	// A refusal leaves the previous airport in place.
	if got := s.GetHomeRunwayData().Airport; got != "LFPZ" {
		t.Errorf("a refused change must not touch the home airport, got %s", got)
	}
}

func TestCandidatesAreNearestFirstAndUsable(t *testing.T) {
	s := newParisService(t, "LFPZ")
	c := s.HomeAirportCandidates(30)
	if len(c) < 3 {
		t.Fatalf("expected several airports within 30 NM of Paris, got %d", len(c))
	}
	seen := map[string]bool{}
	for i, a := range c {
		seen[a.Code] = true
		if a.Type == "heliport" || a.Runways == 0 {
			t.Errorf("%s (%s, %d runways) should not be offered", a.Code, a.Type, a.Runways)
		}
		if a.DistanceNM > 30 {
			t.Errorf("%s at %.1f NM is beyond the asked radius", a.Code, a.DistanceNM)
		}
		if i > 0 && a.DistanceNM < c[i-1].DistanceNM {
			t.Errorf("not sorted by distance at %s", a.Code)
		}
	}
	for _, want := range []string{"LFPO", "LFPG"} {
		if !seen[want] {
			t.Errorf("%s should be offered", want)
		}
	}
}

// Run with -race: the panel changes the airport while the map reads runways.
func TestChangingHomeIsSafeWhileRead(t *testing.T) {
	s := newParisService(t, "LFPZ")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = s.GetHomeRunwayData()
			_ = s.GetHomeRunwayExtensions()
			_ = s.HomeAirportCode()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if i%2 == 0 {
				_, _, _ = s.SetHomeAirport("LFPO")
			} else {
				_, _, _ = s.SetHomeAirport("LFPZ")
			}
		}
	}()
	wg.Wait()
}
