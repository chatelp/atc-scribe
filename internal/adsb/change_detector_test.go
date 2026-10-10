package adsb

import (
	"testing"

	"github.com/yegors/co-atc/pkg/logger"
)

func TestASquawkChangeIsInTheDelta(t *testing.T) {
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	cd := NewChangeDetector(log)
	lat, lon := 48.8, 2.0
	cd.DetectChanges([]*Aircraft{{Hex: "abc123", ADSB: &ADSBTarget{Lat: &lat, Lon: &lon, Squawk: "1000"}}})

	changes := cd.DetectChanges([]*Aircraft{{Hex: "abc123", ADSB: &ADSBTarget{Lat: &lat, Lon: &lon, Squawk: "7700"}}})
	if len(changes) != 1 || changes[0].Type != "updated" {
		t.Fatalf("changes = %+v, want one update", changes)
	}
	if got := changes[0].Delta["squawk"]; got != "7700" {
		t.Fatalf("delta squawk = %v, want 7700", got)
	}

	if changes := cd.DetectChanges([]*Aircraft{{Hex: "abc123", ADSB: &ADSBTarget{Lat: &lat, Lon: &lon, Squawk: "7700"}}}); len(changes) != 0 {
		t.Fatalf("an unchanged squawk gave %+v", changes)
	}
}
