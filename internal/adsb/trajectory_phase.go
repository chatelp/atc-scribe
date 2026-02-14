package adsb

import (
	"math"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// ─── Phase Detection ──────────────────────────────────────────────────────────
//
// DeterminePhase replaces the old single-point determineFlightPhase() function.
// It analyzes the trajectory window to classify the aircraft into one of 10 phases:
//
//   NEW — Newly detected or stationary on ground (no data or parked)
//   TAX — Taxiing on ground (moving at low speed)
//   T/O — Takeoff (handled upstream by detectGroundStateTransitions, not here)
//   CLB — Initial climb out from our airport (runway heading, accelerating, near airport)
//   DEP — Departure / general climb (climbing below cruise, heading away from station)
//   CRZ — Cruise / en-route (at or above FL180)
//   ARR — Arrival (approaching station, below cruise, descending or level)
//   APP — Approach (on final, not climbing, runway-aligned, approaching station)
//   T/D — Touchdown (handled upstream by detectGroundStateTransitions, not here)
//   UNK — Unknown (airborne but doesn't match any specific condition)
//
// CLB and DEP mirror the APP/ARR pair on the departure side:
//   CLB = initial climb from our airport (runway track, accelerating, near airport) — like APP
//   DEP = general departure climb away from station (any climbing outbound traffic) — like ARR
//
// T/O and T/D are detected by the priority-1 ground state transition system
// (detectGroundStateTransitions) and are never emitted by this function.
//
// Rules are evaluated in strict priority order. The first matching rule wins.

// DeterminePhase returns the detected flight phase for the given aircraft.
// It uses the trajectory-derived state for robust, noise-resistant classification.
// When insufficient trajectory data is available (fewer than MinPointsForAnalysis),
// it applies simple heuristics based on the available data points.
func (tt *TrajectoryTracker) DeterminePhase(
	aircraft *Aircraft,
	currentPhase *PhaseChange,
	takeoffTime *time.Time,
) string {
	// ── Rule 0: No data ──────────────────────────────────────────────
	if aircraft.ADSB == nil {
		return "NEW"
	}

	// Ensure derived state is up to date
	derived := tt.EnsureDerived(aircraft.Hex)

	// If we have no trajectory data at all (first ever observation), use
	// minimal heuristics from the single available data point.
	if derived == nil || derived.ValidPointCount == 0 {
		return tt.singlePointPhase(aircraft)
	}

	// ── Rule 1: Ground phases ────────────────────────────────────────
	if aircraft.OnGround {
		return tt.ruleGround(aircraft, currentPhase, derived)
	}

	// ── Airborne rules (require trajectory analysis) ─────────────────

	// If we have fewer points than the analysis threshold, use simplified
	// airborne heuristics that work with limited data.
	if derived.ValidPointCount < tt.config.MinPointsForAnalysis {
		return tt.fewPointsAirbornePhase(aircraft, derived, currentPhase, takeoffTime)
	}

	// ── Full trajectory-based rules ──────────────────────────────────

	// Rule 2: Cruise (highest airborne priority)
	if phase := tt.ruleCruise(derived); phase != "" {
		return phase
	}

	// Rule 3: Approach (specific — runway-aligned final)
	if phase := tt.ruleApproach(aircraft, derived); phase != "" {
		return phase
	}

	// Rule 4: Climb (specific — initial climb out from our airport)
	if phase := tt.ruleClimb(aircraft, derived, takeoffTime); phase != "" {
		return phase
	}

	// Rule 5: Departure (general — climbing away from station)
	if phase := tt.ruleDeparture(derived); phase != "" {
		return phase
	}

	// Rule 6: Arrival (general — approaching station, descending or level)
	if phase := tt.ruleArrival(derived); phase != "" {
		return phase
	}

	// Rule 7: Unknown (catch-all for airborne aircraft that don't match any pattern)
	return "UNK"
}

// ─── Individual Phase Rules ───────────────────────────────────────────────────

// ruleGround classifies ground-phase aircraft.
//
// Conditions:
//   - TAX: Mean ground speed within taxi range (1–50 kts by default)
//   - TAX (preserved): Previous phase was TAX and aircraft briefly stopped
//   - NEW: Everything else (parked, stationary, no prior phase)
func (tt *TrajectoryTracker) ruleGround(aircraft *Aircraft, currentPhase *PhaseChange, d *DerivedState) string {
	cfg := tt.phasesConfig

	// Taxiing: ground speed in taxi range
	if d.GSMean >= float64(cfg.TaxiingMinSpeedKts) && d.GSMean <= float64(cfg.TaxiingMaxSpeedKts) {
		return "TAX"
	}

	// Preserve TAX phase if aircraft briefly stopped (prevents flapping)
	if currentPhase != nil && currentPhase.Phase == "TAX" {
		return "TAX"
	}

	return "NEW"
}

// ruleCruise detects cruise phase (en-route).
//
// Condition: mean altitude at or above cruise altitude threshold (FL180 by default).
//
// No level-flight requirement. FL180 is the boundary of Class A airspace — above it,
// an aircraft is in the en-route phase whether it's still climbing to assigned altitude,
// level at cruise, or beginning initial descent. DEP covers the terminal climb below
// FL180; CRZ takes over once the aircraft enters Class A. This eliminates the gap where
// aircraft climbing through FL180+ would fall to UNK because they weren't yet level.
func (tt *TrajectoryTracker) ruleCruise(d *DerivedState) string {
	if d.AltMean >= float64(tt.phasesConfig.CruiseAltitudeFt) {
		return "CRZ"
	}
	return ""
}

// ruleApproach detects aircraft on final approach to land.
//
// Conditions (ALL must be true):
//   - Below approach_max_altitude_ft (default 5000 ft)
//   - Not climbing (descending or level — covers localizer intercept before glideslope)
//   - Closing on station (distance decreasing)
//   - Aligned with a runway centerline (within centerline tolerance + max distance)
//   - Heading toward the airport (bearing check)
//
// The descent requirement is deliberately relaxed to "not climbing" rather than
// "must be descending." In real operations, an aircraft is on approach once established
// on the localizer, even before glideslope intercept when it may still be level.
// Requiring a 30-second established descent trend (IsDescending) would delay APP
// detection by 30+ seconds after the aircraft turns final — unacceptable for ATC.
func (tt *TrajectoryTracker) ruleApproach(aircraft *Aircraft, d *DerivedState) string {
	cfg := tt.phasesConfig
	adsb := aircraft.ADSB
	lat, lon, hasPosition := adsb.Position()
	if !hasPosition {
		return ""
	}

	// Must be below the approach altitude ceiling
	if d.AltMean > float64(cfg.ApproachMaxAltitudeFt) {
		return ""
	}

	// Must not be climbing — descending or level are both valid approach states.
	// An aircraft established on the localizer before glideslope intercept is level,
	// and that's still "on approach."
	if d.IsClimbing {
		return ""
	}

	// Must be approaching the station
	if !d.IsApproachingStation {
		return ""
	}

	// Check runway alignment using the latest position
	runwayInfo := DetectRunwayApproach(lat, lon, NumberOrZero(adsb.Track), d.AltMean, tt.runwayData, *cfg)
	if runwayInfo == nil || !runwayInfo.OnApproach {
		return ""
	}

	// Filter by active runway — suppress approaches to non-active runways
	// (e.g. perpendicular cross-runway during a base turn). During startup
	// grace (no data), IsActiveRunway returns true for all.
	if tt.runwayTracker != nil && !tt.runwayTracker.IsActiveRunway(runwayInfo.RunwayID) {
		tt.logger.Debug("Approach rejected (runway not active)",
			logger.String("hex", aircraft.Hex),
			logger.String("runway", runwayInfo.RunwayID),
		)
		return ""
	}

	// Verify heading toward airport (bearing difference <= 90°)
	bearingToStation := CalculateBearing(lat, lon, tt.stationLat, tt.stationLon)
	headingDiff := math.Abs(NumberOrZero(adsb.Track) - bearingToStation)
	if headingDiff > 180 {
		headingDiff = 360 - headingDiff
	}
	if headingDiff > 90 {
		return ""
	}

	// Record evidence for runway-in-use detection
	if tt.runwayTracker != nil {
		tt.runwayTracker.RecordEvent(runwayInfo.RunwayID, RunwayEventApproach, aircraft.Hex)
	}

	return "APP"
}

// ruleClimb detects the initial climb out from our airport — the departure-side
// counterpart to ruleApproach. Where APP requires runway alignment on the inbound
// side, CLB requires evidence of a recent takeoff plus initial-climb indicators:
// runway departure heading, acceleration, or close proximity to the airport.
//
// Two paths to CLB:
//
// Path A (observed takeoff): Recent T/O record in DB + climbing + any indicator:
//
//	a) On runway departure heading
//	b) Accelerating and departing from station
//	c) Still within airport vicinity
//
// Path B (inferred takeoff): No T/O record, but aircraft appeared airborne
// climbing on runway heading at low altitude near the airport. With spotty ADS-B
// coverage, aircraft often first appear at ~1000 ft already climbing on the runway
// centerline — this is an obvious takeoff that we missed on the ground.
//
// Once the aircraft turns off the SID heading, stops accelerating, and leaves the
// airport area, CLB conditions no longer match and the aircraft transitions to DEP.
func (tt *TrajectoryTracker) ruleClimb(aircraft *Aircraft, d *DerivedState, takeoffTime *time.Time) string {
	cfg := tt.phasesConfig

	// Must be climbing
	if !d.IsClimbing {
		return ""
	}

	// Must be below cruise altitude
	if d.AltMean >= float64(cfg.CruiseAltitudeFt) {
		return ""
	}

	// Check runway departure heading (used by both paths)
	adsb := aircraft.ADSB
	var departureInfo *RunwayDepartureInfo
	if adsb != nil {
		lat, lon, hasPosition := adsb.Position()
		if hasPosition {
			departureInfo = DetectRunwayDeparture(
				lat, lon, NumberOrZero(adsb.Track),
				tt.runwayData, tt.stationLat, tt.stationLon, *cfg,
			)
		}
	}

	// ── Path A: Observed takeoff ──
	if tt.hasRecentTakeoff(aircraft, takeoffTime) {
		// (a) On runway departure heading
		if departureInfo != nil && departureInfo.OnDeparture {
			if tt.runwayTracker != nil {
				tt.runwayTracker.RecordEvent(departureInfo.RunwayID, RunwayEventClimb, aircraft.Hex)
			}
			return "CLB"
		}

		// (b) Accelerating and departing from station
		if d.IsAccelerating && d.DistTrendNMPerSec > 0.001 {
			return "CLB"
		}

		// (c) Still within airport vicinity
		if d.DistToStationNM <= cfg.AirportRangeNM {
			return "CLB"
		}
	}

	// ── Path B: Inferred takeoff (appeared airborne, no T/O record) ──
	// Aircraft climbing on runway departure heading, at low altitude, near the
	// airport. With spotty ADS-B coverage this is the most common CLB scenario.
	if departureInfo != nil && departureInfo.OnDeparture &&
		d.AltMean <= float64(cfg.DepartureAltitudeFt) &&
		d.DistToStationNM <= cfg.AirportRangeNM {
		if tt.runwayTracker != nil {
			tt.runwayTracker.RecordEvent(departureInfo.RunwayID, RunwayEventClimb, aircraft.Hex)
		}
		return "CLB"
	}

	return ""
}

// ruleDeparture detects aircraft in a general departure climb — climbing below
// cruise altitude and heading away from the station. This is the outbound counterpart
// to ruleArrival: where ARR catches inbound traffic descending toward the station,
// DEP catches outbound traffic climbing away from it.
//
// Conditions (ALL must be true):
//   - Climbing (trajectory trend)
//   - Below cruise altitude
//   - Departing from station (distance increasing)
//
// DEP covers: our own departures after the initial climb (CLB → DEP transition),
// transit traffic from nearby airports, departures where T/O was missed, and any
// other climbing-away aircraft. CLB (higher priority) claims the initial climb out;
// DEP handles everything from there to FL180.
func (tt *TrajectoryTracker) ruleDeparture(d *DerivedState) string {
	// Must be climbing
	if !d.IsClimbing {
		return ""
	}

	// Must be below cruise altitude
	if d.AltMean >= float64(tt.phasesConfig.CruiseAltitudeFt) {
		return ""
	}

	// Must be departing from station
	if d.DistTrendNMPerSec <= 0.001 {
		return ""
	}

	return "DEP"
}

// ruleArrival detects aircraft arriving at the station's airport.
// This is NOT a catch-all — it requires the aircraft to be genuinely approaching.
//
// Conditions (ALL must be true):
//   - Approaching station (distance decreasing over trajectory window)
//   - Below cruise altitude
//   - Descending OR level (not climbing — a climbing aircraft approaching is likely
//     a missed approach/go-around, which should be UNK or DEP)
func (tt *TrajectoryTracker) ruleArrival(d *DerivedState) string {
	cruiseAlt := float64(tt.phasesConfig.CruiseAltitudeFt)

	// Must be approaching the station
	if !d.IsApproachingStation {
		return ""
	}

	// Must be below cruise altitude
	if d.AltMean >= cruiseAlt {
		return ""
	}

	// Must be descending or level (not climbing)
	if d.IsClimbing {
		return ""
	}

	return "ARR"
}

// ─── Simplified Heuristics for Limited Data ───────────────────────────────────

// singlePointPhase provides a best-effort phase classification when only a single
// data point is available (aircraft just appeared). Uses simple altitude/speed
// thresholds without trajectory analysis.
func (tt *TrajectoryTracker) singlePointPhase(aircraft *Aircraft) string {
	if aircraft.ADSB == nil {
		return "NEW"
	}
	if aircraft.OnGround {
		adsb := aircraft.ADSB
		cfg := tt.phasesConfig
		groundSpeed := NumberOrZero(adsb.GS)
		if groundSpeed >= float64(cfg.TaxiingMinSpeedKts) && groundSpeed <= float64(cfg.TaxiingMaxSpeedKts) {
			return "TAX"
		}
		return "NEW"
	}

	// Airborne with just one data point — use altitude as primary discriminator.
	//
	// First observations are often weak Mode S contacts with incomplete data:
	// Track=0 (no heading), AltBaro=0 (no altitude), Lat/Lon=0 (no position).
	// We must guard against treating these zero values as real measurements.
	adsb := aircraft.ADSB
	alt := adsb.AltBaro.Float64()
	cfg := tt.phasesConfig
	cruiseAlt := float64(cfg.CruiseAltitudeFt)
	hasPosition := adsb.HasPosition()
	hasTrack := NumberOrZero(adsb.Track) != 0 || NumberOrZero(adsb.MagHeading) != 0 || NumberOrZero(adsb.TrueHeading) != 0
	hasAltitude := alt > 0

	if hasAltitude && alt >= cruiseAlt {
		return "CRZ"
	}

	// Low altitude, near airport, on runway departure heading → likely CLB.
	// Requires real position, heading, and altitude data (not Mode S zeros).
	// When the active runway is known, also accept aircraft on its heading
	// even without full runway geometry match — very strong signal.
	if hasPosition && hasTrack && hasAltitude && alt <= float64(cfg.DepartureAltitudeFt) {
		lat, lon, _ := adsb.Position()
		track := NumberOrZero(adsb.Track)
		if track == 0 && NumberOrZero(adsb.MagHeading) != 0 {
			track = NumberOrZero(adsb.MagHeading)
		}
		distToStation := MetersToNM(Haversine(lat, lon, tt.stationLat, tt.stationLon))
		if distToStation <= cfg.AirportRangeNM {
			departureInfo := DetectRunwayDeparture(
				lat, lon, track,
				tt.runwayData, tt.stationLat, tt.stationLon, *cfg,
			)
			if departureInfo != nil && departureInfo.OnDeparture {
				// Extra confidence: matches the known active runway
				if tt.runwayTracker != nil && tt.runwayTracker.IsActiveRunway(departureInfo.RunwayID) {
					return "CLB"
				}
				// No active runway data yet — still accept if geometry matches
				return "CLB"
			}
		}
	}

	// Below cruise, single data point — we can't determine trend, so UNK
	return "UNK"
}

// fewPointsAirbornePhase provides phase classification when we have 2-4 data points.
// This is a transitional period — we have some trend information but not enough for
// full trajectory analysis. We use a simplified version of the rules.
func (tt *TrajectoryTracker) fewPointsAirbornePhase(
	aircraft *Aircraft,
	d *DerivedState,
	currentPhase *PhaseChange,
	takeoffTime *time.Time,
) string {
	cruiseAlt := float64(tt.phasesConfig.CruiseAltitudeFt)

	// Cruise is unambiguous even with few points
	if d.AltMean >= cruiseAlt {
		return "CRZ"
	}

	// Recent takeoff from our airport and climbing = initial climb out
	if tt.hasRecentTakeoff(aircraft, takeoffTime) && d.VRMean > 0 {
		return "CLB"
	}

	// Inferred takeoff: climbing on runway heading, low altitude, near airport.
	// With spotty ADS-B coverage, aircraft often first appear already airborne.
	// Use MagHeading as fallback when Track is 0 (common with early Mode S contacts).
	if d.VRMean > 0 && d.AltMean <= float64(tt.phasesConfig.DepartureAltitudeFt) &&
		d.DistToStationNM <= tt.phasesConfig.AirportRangeNM && aircraft.ADSB != nil {
		lat, lon, hasPosition := aircraft.ADSB.Position()
		if !hasPosition {
			return "UNK"
		}
		track := NumberOrZero(aircraft.ADSB.Track)
		if track == 0 && NumberOrZero(aircraft.ADSB.MagHeading) != 0 {
			track = NumberOrZero(aircraft.ADSB.MagHeading)
		}
		if track != 0 {
			departureInfo := DetectRunwayDeparture(
				lat, lon, track,
				tt.runwayData, tt.stationLat, tt.stationLon, *tt.phasesConfig,
			)
			if departureInfo != nil && departureInfo.OnDeparture {
				return "CLB"
			}
		}
	}

	// Climbing and moving away from station = general departure
	if d.VRMean > 0 && d.DistTrendNMPerSec > 0.001 {
		return "DEP"
	}

	// Approaching station and descending
	if d.IsApproachingStation && d.VRMean < 0 && d.AltMean < cruiseAlt {
		return "ARR"
	}

	return "UNK"
}

// hasRecentTakeoff determines if an aircraft has taken off from our airport recently.
// Only returns true if we actually observed the takeoff (T/O phase record in DB).
// No proximity heuristic — aircraft appearing mid-flight near the airport are NOT
// treated as recent takeoffs.
func (tt *TrajectoryTracker) hasRecentTakeoff(aircraft *Aircraft, takeoffTime *time.Time) bool {
	cfg := tt.phasesConfig
	timeoutDuration := time.Duration(cfg.RecentTakeoffTimeoutMinutes) * time.Minute

	// Only trust actual observed takeoff records from the database
	if takeoffTime != nil && time.Since(*takeoffTime) <= timeoutDuration {
		return true
	}

	return false
}

// ─── Signal-Lost Landing Enhancement ──────────────────────────────────────────

// WasDescendingTowardStation checks the trajectory history to determine if an
// aircraft was on a descending trajectory toward the station before signal was lost.
// Returns true with high confidence when the medium-window altitude trend shows
// steady descent and the aircraft was closing on the station.
//
// This is used by detectSignalLostLandings() to boost confidence in auto-landing
// inference when an aircraft disappears near the airport.
func (tt *TrajectoryTracker) WasDescendingTowardStation(hex string) bool {
	tt.mu.RLock()
	at, ok := tt.aircraft[hex]
	tt.mu.RUnlock()
	if !ok || at.Count < 3 {
		return false
	}

	// Ensure derived state is fresh
	if at.DirtyDerived {
		tt.computeDerivedState(at)
		at.DirtyDerived = false
	}

	d := &at.Derived

	// Check: was descending AND approaching station
	return d.IsDescending && d.IsApproachingStation && d.AltTrendFPM < -100

	// Note: We use a gentler threshold (-100 fpm) than the normal descent threshold
	// (-200 fpm) because near landing the descent rate may be quite shallow.
}

// ─── Logging ──────────────────────────────────────────────────────────────────

// LogDerivedState logs the key derived state fields for debugging.
func (tt *TrajectoryTracker) LogDerivedState(hex string) {
	d := tt.EnsureDerived(hex)
	if d == nil {
		return
	}
	tt.logger.Debug("Trajectory derived state",
		logger.String("hex", hex),
		logger.Int("valid_points", d.ValidPointCount),
		logger.Float64("alt_mean", d.AltMean),
		logger.Float64("alt_trend_fpm", d.AltTrendFPM),
		logger.Float64("gs_mean", d.GSMean),
		logger.Float64("gs_trend", d.GSTrendKtsPerSec),
		logger.Float64("vr_mean", d.VRMean),
		logger.Float64("dist_nm", d.DistToStationNM),
		logger.Float64("dist_trend", d.DistTrendNMPerSec),
		logger.Bool("descending", d.IsDescending),
		logger.Bool("climbing", d.IsClimbing),
		logger.Bool("level", d.IsLevel),
		logger.Bool("approaching", d.IsApproachingStation),
		logger.Bool("turning", d.IsTurning),
	)
}
