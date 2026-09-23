package adsb

import (
	"sync"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/pkg/logger"
)

// The geometry is synthetic on purpose: a repository must not carry a real
// receiver's position. What matters is the arrangement, which is the one that
// broke upstream's phases -- a receiver about 14 NM west of the airport it
// watches, with an east-west runway.
const (
	airportLat, airportLon   = 45.0, 5.0
	receiverLat, receiverLon = 45.0, 4.67 // ~14 NM west at this latitude
)

// Runway 09/27: land on 09 heading east, on 27 heading west.
func testRunways() RunwayData {
	rd := RunwayData{Airport: "ZZZZ", RunwayThresholds: map[string]map[string]struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}{}}
	rd.RunwayThresholds["09-27"] = map[string]struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}{
		"09": {Latitude: airportLat, Longitude: 4.985},
		"27": {Latitude: airportLat, Longitude: 5.015},
	}
	return rd
}

// The values of the station's own [flight_phases] section.
func testPhasesConfig() *config.FlightPhasesConfig {
	return &config.FlightPhasesConfig{
		Enabled: true, CruiseAltitudeFt: 18000, DepartureAltitudeFt: 1000,
		TaxiingMinSpeedKts: 1, TaxiingMaxSpeedKts: 50,
		ApproachCenterlineToleranceNM: 0.5, ApproachMaxDistanceNM: 10,
		ApproachMaxAltitudeFt: 5000, ApproachHeadingToleranceDeg: 30,
		RecentTakeoffTimeoutMinutes: 30, AirportRangeNM: 5,
		FlyingMinTASKts: 50, FlyingMinAltFt: 300,
		RunwayInUseWindowMinutes: 60, RunwayInUseApproachWeight: 2,
		RunwayInUseLandingWeight: 3, RunwayInUseClimbWeight: 2, RunwayInUseDecayRate: 0.98,
	}
}

func newTestTracker(t *testing.T, ref PhaseReference) *TrajectoryTracker {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	tt := NewTrajectoryTracker(TrajectoryConfig{
		BufferDurationSec: 90, BufferCapacity: 100, FetchIntervalSec: 1,
		MinPointsForAnalysis: 5, StaleTimeoutSec: 300, CleanupIntervalSec: 30,
		DescentVRThresholdFPM: -200, ClimbVRThresholdFPM: 200, LevelAltBandFt: 200,
		TurningRateThresholdDeg: 1.5, DecelerationThreshold: -0.5, AccelerationThreshold: 0.5,
	}, newPhaseRef(ref), testPhasesConfig(), log)
	t.Cleanup(tt.Stop)
	return tt
}

// fly feeds a straight, constant-rate trajectory one second at a time and
// returns the aircraft as it stands at the last point.
func fly(tt *TrajectoryTracker, hex string, lon0, lon1, alt0, alt1, track, gs float64) *Aircraft {
	const n = 30
	start := time.Now().Add(-n * time.Second)
	vr := (alt1 - alt0) / float64(n) * 60 // fpm
	var last TrajectorySnapshot
	for i := 0; i <= n; i++ {
		f := float64(i) / n
		last = TrajectorySnapshot{
			Timestamp: start.Add(time.Duration(i) * time.Second),
			Lat:       airportLat, Lon: lon0 + f*(lon1-lon0),
			AltBaro: alt0 + f*(alt1-alt0), GS: gs, TAS: gs, Track: track,
			BaroRate: vr, Valid: true,
		}
		tt.Ingest(hex, last)
	}
	lat, lon, alt := last.Lat, last.Lon, last.AltBaro
	return &Aircraft{Hex: hex, ADSB: &ADSBTarget{
		Hex: hex, Lat: &lat, Lon: &lon, AltBaro: FlexibleFloat64(alt),
		Track: &track, GS: &gs, TAS: &gs, BaroRate: &vr,
	}}
}

// Final approach to runway 09 from the west: towards the airport, away from the
// receiver. Upstream required "approaching the station" and never saw it.
func approachFromWest(tt *TrajectoryTracker) string {
	ac := fly(tt, "a00001", 4.90, 4.97, 2500, 1500, 90, 140)
	return tt.DeterminePhase(ac, nil, nil)
}

// Initial climb off runway 27, heading west: away from the airport, towards the
// receiver. Upstream required "moving away from the station" and "within 5 NM of
// the station", and failed both.
func departureWestbound(tt *TrajectoryTracker) string {
	ac := fly(tt, "a00002", 4.975, 4.955, 300, 900, 270, 150)
	return tt.DeterminePhase(ac, nil, nil)
}

func TestApproachIsJudgedAgainstTheAirport(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{
		Airport: "ZZZZ", Lat: airportLat, Lon: airportLon, Runways: testRunways()})
	if got := approachFromWest(tt); got != "APP" {
		t.Fatalf("an approach to the reference airport should be APP, got %s", got)
	}
}

func TestInitialClimbIsJudgedAgainstTheAirport(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{
		Airport: "ZZZZ", Lat: airportLat, Lon: airportLon, Runways: testRunways()})
	if got := departureWestbound(tt); got != "CLB" {
		t.Fatalf("a climb out of the reference airport should be CLB, got %s", got)
	}
}

// The control: the same trajectories, measured from the receiver as upstream did.
// If these passed, the two tests above would prove nothing.
func TestMeasuredFromTheReceiverTheyAreMissed(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{
		Airport: "ZZZZ", Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
	if got := approachFromWest(tt); got == "APP" {
		t.Errorf("measured from the receiver, the approach should be missed, got APP")
	}
	if got := departureWestbound(tt); got == "CLB" {
		t.Errorf("measured from the receiver, the climb should be missed, got CLB")
	}
}

// Changing the reference while running -- the settings panel does exactly this.
func TestReferenceCanChangeWhileRunning(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
	if got := approachFromWest(tt); got == "APP" {
		t.Fatalf("precondition: from the receiver the approach is missed, got APP")
	}
	tt.ref.store(PhaseReference{Airport: "ZZZZ", Lat: airportLat, Lon: airportLon, Runways: testRunways()})
	tt.referenceChanged()
	if got := approachFromWest(tt); got != "APP" {
		t.Fatalf("after switching to the airport the approach should be APP, got %s", got)
	}
}

// The evidence of which runway is in use belonged to the previous airport.
func TestChangingReferenceForgetsRunwayEvidence(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{
		Airport: "ZZZZ", Lat: airportLat, Lon: airportLon, Runways: testRunways()})
	approachFromWest(tt) // records an approach on 09
	if !tt.runwayTracker.HasData() {
		t.Fatal("precondition: the approach should have left runway evidence")
	}
	tt.referenceChanged()
	if tt.runwayTracker.HasData() {
		t.Error("runway evidence should be dropped when the reference changes")
	}
	if !tt.runwayTracker.IsActiveRunway("anything") {
		t.Error("with no evidence left, every runway should be accepted again")
	}
}

// Run with -race: the fetch goroutine classifies while the panel swaps airports.
func TestSwappingReferenceIsSafeWhileClassifying(t *testing.T) {
	tt := newTestTracker(t, PhaseReference{Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
	ref := tt.ref
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			approachFromWest(tt)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if i%2 == 0 {
				ref.store(PhaseReference{Lat: airportLat, Lon: airportLon, Runways: testRunways()})
			} else {
				ref.store(PhaseReference{Lat: receiverLat, Lon: receiverLon, Runways: testRunways()})
			}
			tt.referenceChanged()
		}
	}()
	wg.Wait()
}
