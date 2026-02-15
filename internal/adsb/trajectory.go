// Package adsb provides ADS-B aircraft tracking, trajectory analysis, and flight phase detection.
//
// # Trajectory-Based Phase Detection
//
// The TrajectoryTracker maintains a rolling window of recent ADS-B observations for each
// tracked aircraft. Instead of making phase decisions from a single data point (which is
// fragile near coverage edges), the system analyzes trajectory windows to compute derived
// state vectors—velocity trends, acceleration, altitude slopes, and distance-to-station
// rates—that enable robust, noise-resistant flight phase classification.
//
// Each aircraft's ring buffer holds ~90 seconds of data at 1-second fetch intervals.
// The derived state is recomputed once per fetch cycle using three analysis windows:
//
//   - Short (10s): Current velocity and acceleration. Responds quickly to real changes.
//   - Medium (30s): Trend detection (climbing, descending, turning). Smooths out noise.
//   - Long (full buffer): High-confidence decisions and gap detection.
//
// Key algorithms:
//   - Ordinary Least Squares (OLS) linear regression for altitude/speed/distance trends
//   - Median filtering for vertical rate (resists outlier spikes from bad transponder data)
//   - Circular difference for heading rate (handles 0°/360° wraparound)
//   - Data gap detection to avoid corrupting trends with stale data
//
// The ring buffer is pre-allocated and reuses slots, producing zero GC pressure during
// steady-state operation. Memory cost is ~18 KB per aircraft, ~9 MB for 500 aircraft.

package adsb

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/pkg/logger"
)

const maxLivePredictionAge = 60 * time.Second

// ─── Trajectory Snapshot ──────────────────────────────────────────────────────

// TrajectorySnapshot is a single ADS-B observation stored in the ring buffer.
// Fields are a flattened subset of ADSBTarget, with a proper time.Time timestamp
// and a validity flag. Invalid snapshots (no position data) are stored but excluded
// from derived state computations.
type TrajectorySnapshot struct {
	Timestamp   time.Time // UTC wall-clock time when this observation was ingested
	Lat         float64   // Latitude (degrees)
	Lon         float64   // Longitude (degrees)
	AltBaro     float64   // Barometric altitude (feet)
	AltGeom     float64   // Geometric altitude (feet)
	GS          float64   // Ground speed (knots)
	TAS         float64   // True airspeed (knots)
	IAS         float64   // Indicated airspeed (knots)
	Track       float64   // Track angle (degrees, 0-360)
	MagHeading  float64   // Magnetic heading (degrees)
	TrueHeading float64   // True heading (degrees)
	BaroRate    float64   // Barometric vertical rate (feet per minute)
	GeomRate    float64   // Geometric vertical rate (feet per minute)
	Roll        float64   // Roll angle (degrees)
	TrackRate   float64   // Rate of change of track (degrees per second)
	OnGround    bool      // Whether the aircraft is on the ground
	NavAltMCP   float64   // MCP (Mode Control Panel) altitude setting (feet)
	NavAltFMS   float64   // FMS (Flight Management System) altitude setting (feet)
	Seen        float64   // Seconds since last ADS-B message (data freshness)
	Valid       bool      // False if key fields are missing (no position)
}

// TrajectorySnapshotFromADSB converts an ADSBTarget and ground state into a
// TrajectorySnapshot suitable for the ring buffer. The snapshot is marked invalid
// when both lat and lon are zero (no position data available).
func TrajectorySnapshotFromADSB(adsb *ADSBTarget, onGround bool, ts time.Time) TrajectorySnapshot {
	if adsb == nil {
		return TrajectorySnapshot{Timestamp: ts, Valid: false}
	}
	lat, lon, hasPosition := adsb.Position()
	return TrajectorySnapshot{
		Timestamp:   ts,
		Lat:         lat,
		Lon:         lon,
		AltBaro:     adsb.AltBaro.Float64(),
		AltGeom:     adsb.AltGeom.Float64(),
		GS:          NumberOrZero(adsb.GS),
		TAS:         NumberOrZero(adsb.TAS),
		IAS:         NumberOrZero(adsb.IAS),
		Track:       NumberOrZero(adsb.Track),
		MagHeading:  NumberOrZero(adsb.MagHeading),
		TrueHeading: NumberOrZero(adsb.TrueHeading),
		BaroRate:    NumberOrZero(adsb.BaroRate),
		GeomRate:    NumberOrZero(adsb.GeomRate),
		Roll:        NumberOrZero(adsb.Roll),
		TrackRate:   NumberOrZero(adsb.TrackRate),
		OnGround:    onGround,
		NavAltMCP:   NumberOrZero(adsb.NavAltitudeMCP),
		NavAltFMS:   NumberOrZero(adsb.NavAltitudeFMS),
		Seen:        NumberOrZero(adsb.Seen),
		Valid:       hasPosition,
	}
}

// ─── Derived State ────────────────────────────────────────────────────────────

// DerivedState holds quantities computed from the trajectory window. It is
// recalculated once per fetch cycle when new data arrives (DirtyDerived flag).
// Phase rules operate on these derived values rather than raw instantaneous readings.
type DerivedState struct {
	// Smoothed current state (short window median/mean)
	GroundSpeedKts  float64 // Smoothed ground speed (knots)
	VerticalRateFPM float64 // Smoothed barometric vertical rate (feet per minute)
	TrackDeg        float64 // Smoothed track angle (degrees)

	// Acceleration (rate of change over short window)
	GSAccelKtsPerSec   float64 // d(GS)/dt — positive = accelerating
	VRAccelFPMPerSec   float64 // d(VerticalRate)/dt — positive = increasing climb
	TrackRateDegPerSec float64 // Heading rate of change (degrees per second)

	// Altitude statistics (medium window)
	AltMean     float64 // Mean barometric altitude (feet)
	AltMin      float64 // Minimum barometric altitude (feet)
	AltMax      float64 // Maximum barometric altitude (feet)
	AltTrendFPM float64 // OLS regression slope of altitude vs time, converted to fpm

	// Speed statistics (medium window)
	GSMean           float64 // Mean ground speed (knots)
	GSMin            float64 // Minimum ground speed (knots)
	GSMax            float64 // Maximum ground speed (knots)
	GSTrendKtsPerSec float64 // OLS regression slope of GS vs time (knots per second)

	// Vertical rate statistics (medium window)
	VRMean   float64 // Mean vertical rate (fpm)
	VRStdDev float64 // Standard deviation of vertical rate

	// Distance and bearing to monitoring station
	DistToStationNM   float64 // Current distance to station (nautical miles)
	DistTrendNMPerSec float64 // OLS regression slope of distance vs time (negative = approaching)
	BearingToStation  float64 // Current bearing to station (degrees)

	// Data quality indicators
	ValidPointCount   int     // Number of valid snapshots in the analysis window
	WindowDurationSec float64 // Time span from oldest to newest valid snapshot (seconds)
	DataGapDetected   bool    // True if any gap > 2× fetch interval found in the window

	// Boolean flags derived from trends (set by ComputeDerivedState)
	IsDescending         bool // AltTrendFPM below descent threshold AND VRMean < 0
	IsClimbing           bool // AltTrendFPM above climb threshold AND VRMean > 0
	IsLevel              bool // (AltMax - AltMin) within level band over medium window
	IsDecelerating       bool // GSTrendKtsPerSec below deceleration threshold
	IsAccelerating       bool // GSTrendKtsPerSec above acceleration threshold
	IsTurning            bool // |TrackRateDegPerSec| above turning threshold
	IsApproachingStation bool // DistTrendNMPerSec < 0 (closing on station)

	ComputedAt time.Time // When this derived state was last computed
}

// ─── Per-Aircraft Trajectory ──────────────────────────────────────────────────

// AircraftTrajectory holds the ring buffer and derived state for a single aircraft.
// The ring buffer is a fixed-capacity slice that overwrites the oldest entry when full,
// avoiding allocations during steady-state operation.
type AircraftTrajectory struct {
	Hex          string               // ICAO hex identifier
	Snapshots    []TrajectorySnapshot // Ring buffer (fixed capacity)
	WriteIdx     int                  // Next write position
	Count        int                  // Number of entries stored (up to capacity)
	Derived      DerivedState         // Latest derived state
	Prediction   TrajectoryPrediction // Hindcast + forecast predictions
	DirtyDerived bool                 // True when new snapshots have been added since last compute
	LastSeen     time.Time            // Tracks staleness for cleanup
}

// NewAircraftTrajectory creates a ring buffer with the given capacity.
func NewAircraftTrajectory(hex string, capacity int) *AircraftTrajectory {
	return &AircraftTrajectory{
		Hex:       hex,
		Snapshots: make([]TrajectorySnapshot, capacity),
	}
}

// AddSnapshot writes a snapshot to the ring buffer, advancing the write index.
func (at *AircraftTrajectory) AddSnapshot(snap TrajectorySnapshot) {
	at.Snapshots[at.WriteIdx] = snap
	at.WriteIdx = (at.WriteIdx + 1) % cap(at.Snapshots)
	if at.Count < cap(at.Snapshots) {
		at.Count++
	}
	at.DirtyDerived = true
	at.LastSeen = snap.Timestamp
}

// ForEachSnapshot iterates over all stored snapshots from oldest to newest,
// calling fn for each. This avoids allocating a temporary slice.
func (at *AircraftTrajectory) ForEachSnapshot(fn func(snap *TrajectorySnapshot)) {
	capacity := cap(at.Snapshots)
	startIdx := 0
	if at.Count == capacity {
		startIdx = at.WriteIdx // In a full buffer, write position holds the oldest entry
	}
	for i := 0; i < at.Count; i++ {
		idx := (startIdx + i) % capacity
		fn(&at.Snapshots[idx])
	}
}

// Latest returns a pointer to the most recently added snapshot, or nil if empty.
func (at *AircraftTrajectory) Latest() *TrajectorySnapshot {
	if at.Count == 0 {
		return nil
	}
	idx := (at.WriteIdx - 1 + cap(at.Snapshots)) % cap(at.Snapshots)
	return &at.Snapshots[idx]
}

// SnapshotsInWindow returns valid snapshots within the last windowSec seconds,
// ordered oldest to newest. Allocates a slice — use ForEachSnapshot for hot paths.
func (at *AircraftTrajectory) SnapshotsInWindow(windowSec float64) []TrajectorySnapshot {
	if at.Count == 0 {
		return nil
	}
	latest := at.Latest()
	if latest == nil {
		return nil
	}
	cutoff := latest.Timestamp.Add(-time.Duration(windowSec * float64(time.Second)))
	result := make([]TrajectorySnapshot, 0, at.Count)
	at.ForEachSnapshot(func(snap *TrajectorySnapshot) {
		if snap.Valid && !snap.Timestamp.Before(cutoff) {
			result = append(result, *snap)
		}
	})
	return result
}

// ─── Trajectory Tracker (top-level) ───────────────────────────────────────────

// TrajectoryConfig holds tunable parameters for the trajectory system.
type TrajectoryConfig struct {
	BufferDurationSec    int // How many seconds of history to keep (default: 90)
	BufferCapacity       int // Max snapshots per aircraft (computed from duration / fetch interval + margin)
	FetchIntervalSec     int // ADS-B fetch interval in seconds (from ADSBConfig)
	MinPointsForAnalysis int // Minimum valid points before full trajectory analysis (default: 5)
	StaleTimeoutSec      int // Remove aircraft not seen for this long (default: 300)
	CleanupIntervalSec   int // How often to run the cleanup goroutine (default: 30)

	// Thresholds for derived boolean flags
	DescentVRThresholdFPM   float64 // AltTrend below this = descending (default: -200)
	ClimbVRThresholdFPM     float64 // AltTrend above this = climbing (default: 200)
	LevelAltBandFt          float64 // (AltMax-AltMin) within this = level (default: 200)
	TurningRateThresholdDeg float64 // |TrackRate| above this = turning (default: 1.5)
	DecelerationThreshold   float64 // GSTrend below this = decelerating (default: -0.5)
	AccelerationThreshold   float64 // GSTrend above this = accelerating (default: 0.5)
}

// TrajectoryTracker manages trajectory buffers for all tracked aircraft.
// It is safe for concurrent use: the fetch goroutine calls Ingest and DeterminePhase
// on the same goroutine, while a background cleanup goroutine periodically purges
// stale entries under a write lock.
type TrajectoryTracker struct {
	mu            sync.RWMutex
	aircraft      map[string]*AircraftTrajectory
	config        TrajectoryConfig
	stationLat    float64
	stationLon    float64
	runwayData    RunwayData
	phasesConfig  *config.FlightPhasesConfig
	runwayTracker *RunwayInUseTracker
	logger        *logger.Logger
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewTrajectoryTracker creates and starts the tracker with the given configuration.
// The cleanup goroutine runs in the background until Stop is called.
func NewTrajectoryTracker(
	cfg TrajectoryConfig,
	stationLat, stationLon float64,
	runwayData RunwayData,
	phasesConfig *config.FlightPhasesConfig,
	log *logger.Logger,
) *TrajectoryTracker {
	tt := &TrajectoryTracker{
		aircraft:     make(map[string]*AircraftTrajectory),
		config:       cfg,
		stationLat:   stationLat,
		stationLon:   stationLon,
		runwayData:   runwayData,
		phasesConfig: phasesConfig,
		runwayTracker: NewRunwayInUseTracker(
			phasesConfig.RunwayInUseWindowMinutes,
			phasesConfig.RunwayInUseApproachWeight,
			phasesConfig.RunwayInUseLandingWeight,
			phasesConfig.RunwayInUseClimbWeight,
			phasesConfig.RunwayInUseDecayRate,
			log,
		),
		logger: log.Named("trajectory"),
		stopCh: make(chan struct{}),
	}
	tt.runwayTracker.SetRunwayData(runwayData)
	tt.wg.Add(1)
	go tt.cleanupLoop()
	tt.logger.Info("Trajectory tracker started",
		logger.Int("buffer_duration_sec", cfg.BufferDurationSec),
		logger.Int("buffer_capacity", cfg.BufferCapacity),
		logger.Int("min_points", cfg.MinPointsForAnalysis),
	)
	return tt
}

// Stop shuts down the background cleanup goroutine and waits for it to finish.
func (tt *TrajectoryTracker) Stop() {
	close(tt.stopCh)
	tt.wg.Wait()
	tt.logger.Info("Trajectory tracker stopped")
}

// RecordRunwayLanding records a landing event for runway-in-use detection.
// Called by the service when a T/D is detected near a runway threshold.
func (tt *TrajectoryTracker) RecordRunwayLanding(runwayID string, hex string) {
	if tt.runwayTracker != nil {
		tt.runwayTracker.RecordEvent(runwayID, RunwayEventLanding, hex)
	}
}

// GetRunwayScores returns the top N runway-in-use scores for external consumers.
func (tt *TrajectoryTracker) GetRunwayScores(n int) []RunwayScore {
	if tt.runwayTracker != nil {
		return tt.runwayTracker.GetTopScores(n)
	}
	return nil
}

// Ingest adds a new snapshot for the given aircraft. If the aircraft has no
// buffer yet, one is created. Called once per aircraft per fetch cycle.
func (tt *TrajectoryTracker) Ingest(hex string, snap TrajectorySnapshot) {
	tt.mu.Lock()
	at, ok := tt.aircraft[hex]
	if !ok {
		at = NewAircraftTrajectory(hex, tt.config.BufferCapacity)
		tt.aircraft[hex] = at
	}
	at.AddSnapshot(snap)
	tt.mu.Unlock()
}

// GetTrajectory returns the per-aircraft trajectory, or nil if not tracked.
func (tt *TrajectoryTracker) GetTrajectory(hex string) *AircraftTrajectory {
	tt.mu.RLock()
	at := tt.aircraft[hex]
	tt.mu.RUnlock()
	return at
}

// EnsureDerived recomputes the derived state for the given aircraft if it is
// dirty (new data since last computation). This is called by DeterminePhase
// before reading derived fields.
func (tt *TrajectoryTracker) EnsureDerived(hex string) *DerivedState {
	tt.mu.RLock()
	at, ok := tt.aircraft[hex]
	tt.mu.RUnlock()
	if !ok || at.Count == 0 {
		return nil
	}
	if at.DirtyDerived {
		tt.computeDerivedState(at)
		tt.computePredictions(at)
		at.DirtyDerived = false
	}
	return &at.Derived
}

// computePredictions updates hindcast and forecast for an aircraft.
// Called after computeDerivedState to keep predictions in sync with derived state.
func (tt *TrajectoryTracker) computePredictions(at *AircraftTrajectory) {
	computeHindcast(at, &at.Derived)
	computeForecast(at, &at.Derived)
}

// GetHindcast returns hindcast prediction points for an aircraft, or nil.
func (tt *TrajectoryTracker) GetHindcast(hex string) []PredictionPoint {
	tt.mu.RLock()
	at, ok := tt.aircraft[hex]
	tt.mu.RUnlock()
	if !ok {
		return nil
	}
	return at.Prediction.Hindcast
}

// GetForecast returns forecast prediction points for an aircraft, or nil.
func (tt *TrajectoryTracker) GetForecast(hex string) []PredictionPoint {
	tt.mu.RLock()
	at, ok := tt.aircraft[hex]
	tt.mu.RUnlock()
	if !ok {
		return nil
	}
	return at.Prediction.Forecast
}

// LivePrediction is an interpolated server-side predicted state for a specific aircraft.
// It is intended for high-frequency websocket updates between real ADS-B polls.
type LivePrediction struct {
	Hex          string    `json:"hex"`
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	Altitude     float64   `json:"altitude"`
	Heading      float64   `json:"heading"`
	Speed        float64   `json:"speed"`
	Confidence   float64   `json:"confidence"`
	Timestamp    time.Time `json:"timestamp"`
	BaseObserved time.Time `json:"base_observed"`
}

// GetLivePredictionsAt returns interpolated live prediction states for all tracked aircraft at time t.
// Real ADS-B updates should always supersede these client-side.
func (tt *TrajectoryTracker) GetLivePredictionsAt(t time.Time) []LivePrediction {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	result := make([]LivePrediction, 0, len(tt.aircraft))
	for hex, at := range tt.aircraft {
		if at == nil || at.Count == 0 {
			continue
		}

		latest := at.Latest()
		if latest == nil || !latest.Valid {
			continue
		}
		if t.Sub(latest.Timestamp) > maxLivePredictionAge {
			continue
		}

		forecast := at.Prediction.Forecast
		if len(forecast) == 0 {
			continue
		}

		p, ok := interpolateLivePredictionFromForecast(latest, forecast, t)
		if !ok {
			continue
		}

		p.Hex = hex
		p.BaseObserved = latest.Timestamp
		result = append(result, p)
	}

	return result
}

func interpolateLivePredictionFromForecast(latest *TrajectorySnapshot, forecast []PredictionPoint, atTime time.Time) (LivePrediction, bool) {
	if latest == nil || len(forecast) == 0 {
		return LivePrediction{}, false
	}

	first := forecast[0]
	latestHeading := first.Heading
	if latest.TrueHeading != 0 {
		latestHeading = latest.TrueHeading
	} else if latest.MagHeading != 0 {
		latestHeading = latest.MagHeading
	} else if latest.Track != 0 {
		latestHeading = latest.Track
	}

	if atTime.Before(first.Timestamp) {
		start := latest.Timestamp
		end := first.Timestamp
		if !end.After(start) {
			return LivePrediction{
				Lat:        first.Lat,
				Lon:        first.Lon,
				Altitude:   first.Altitude,
				Heading:    first.Heading,
				Speed:      first.Speed,
				Confidence: first.Confidence,
				Timestamp:  atTime,
			}, true
		}

		alpha := atTime.Sub(start).Seconds() / end.Sub(start).Seconds()
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 1 {
			alpha = 1
		}

		return LivePrediction{
			Lat:        lerpFloat(latest.Lat, first.Lat, alpha),
			Lon:        lerpFloat(latest.Lon, first.Lon, alpha),
			Altitude:   lerpFloat(latest.AltBaro, first.Altitude, alpha),
			Heading:    lerpAngleDeg(latestHeading, first.Heading, alpha),
			Speed:      lerpFloat(latest.GS, first.Speed, alpha),
			Confidence: lerpFloat(0.95, first.Confidence, alpha),
			Timestamp:  atTime,
		}, true
	}

	for i := 0; i < len(forecast)-1; i++ {
		a := forecast[i]
		b := forecast[i+1]
		if atTime.Before(a.Timestamp) || atTime.After(b.Timestamp) {
			continue
		}

		den := b.Timestamp.Sub(a.Timestamp).Seconds()
		if den <= 0 {
			return LivePrediction{}, false
		}
		alpha := atTime.Sub(a.Timestamp).Seconds() / den
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 1 {
			alpha = 1
		}

		return LivePrediction{
			Lat:        lerpFloat(a.Lat, b.Lat, alpha),
			Lon:        lerpFloat(a.Lon, b.Lon, alpha),
			Altitude:   lerpFloat(a.Altitude, b.Altitude, alpha),
			Heading:    lerpAngleDeg(a.Heading, b.Heading, alpha),
			Speed:      lerpFloat(a.Speed, b.Speed, alpha),
			Confidence: lerpFloat(a.Confidence, b.Confidence, alpha),
			Timestamp:  atTime,
		}, true
	}

	last := forecast[len(forecast)-1]
	if atTime.After(last.Timestamp) {
		return LivePrediction{}, false
	}

	return LivePrediction{
		Lat:        last.Lat,
		Lon:        last.Lon,
		Altitude:   last.Altitude,
		Heading:    last.Heading,
		Speed:      last.Speed,
		Confidence: last.Confidence,
		Timestamp:  atTime,
	}, true
}

func lerpFloat(a, b, t float64) float64 {
	return a + (b-a)*t
}

func lerpAngleDeg(a, b, t float64) float64 {
	d := math.Mod((b-a)+540.0, 360.0) - 180.0
	out := a + d*t
	return math.Mod(out+360.0, 360.0)
}

// ─── Cleanup ──────────────────────────────────────────────────────────────────

func (tt *TrajectoryTracker) cleanupLoop() {
	defer tt.wg.Done()
	ticker := time.NewTicker(time.Duration(tt.config.CleanupIntervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tt.cleanup()
		case <-tt.stopCh:
			return
		}
	}
}

func (tt *TrajectoryTracker) cleanup() {
	now := time.Now().UTC()
	staleThreshold := time.Duration(tt.config.StaleTimeoutSec) * time.Second
	tt.mu.Lock()
	removed := 0
	for hex, at := range tt.aircraft {
		if now.Sub(at.LastSeen) > staleThreshold {
			delete(tt.aircraft, hex)
			removed++
		}
	}
	tt.mu.Unlock()
	if removed > 0 {
		tt.logger.Debug("Trajectory cleanup",
			logger.Int("removed", removed),
			logger.Int("remaining", len(tt.aircraft)),
		)
	}
}

// ─── Derived State Computation ────────────────────────────────────────────────

// computeDerivedState recalculates all derived fields from the ring buffer contents.
// This runs once per fetch cycle per aircraft (~90 data points, O(N) single pass).
func (tt *TrajectoryTracker) computeDerivedState(at *AircraftTrajectory) {
	d := &at.Derived
	*d = DerivedState{ComputedAt: time.Now().UTC()} // Reset

	latest := at.Latest()
	if latest == nil || !latest.Valid {
		return
	}

	// Collect valid snapshots and detect data gaps
	shortWindowSec := 10.0
	mediumWindowSec := 30.0
	fetchInterval := float64(tt.config.FetchIntervalSec)
	if fetchInterval < 1 {
		fetchInterval = 1
	}
	gapThreshold := fetchInterval * 2.5

	var (
		allValid    []TrajectorySnapshot
		shortValid  []TrajectorySnapshot
		mediumValid []TrajectorySnapshot
	)

	shortCutoff := latest.Timestamp.Add(-time.Duration(shortWindowSec * float64(time.Second)))
	mediumCutoff := latest.Timestamp.Add(-time.Duration(mediumWindowSec * float64(time.Second)))

	var prevTimestamp time.Time
	gapDetected := false

	at.ForEachSnapshot(func(snap *TrajectorySnapshot) {
		if !snap.Valid {
			return
		}
		allValid = append(allValid, *snap)
		if !snap.Timestamp.Before(mediumCutoff) {
			mediumValid = append(mediumValid, *snap)
		}
		if !snap.Timestamp.Before(shortCutoff) {
			shortValid = append(shortValid, *snap)
		}
		// Gap detection
		if !prevTimestamp.IsZero() {
			gap := snap.Timestamp.Sub(prevTimestamp).Seconds()
			if gap > gapThreshold {
				gapDetected = true
			}
		}
		prevTimestamp = snap.Timestamp
	})

	d.ValidPointCount = len(allValid)
	d.DataGapDetected = gapDetected
	if len(allValid) >= 2 {
		d.WindowDurationSec = allValid[len(allValid)-1].Timestamp.Sub(allValid[0].Timestamp).Seconds()
	}

	if len(allValid) == 0 {
		return
	}

	// ── Current distance and bearing to station ──
	d.DistToStationNM = MetersToNM(Haversine(latest.Lat, latest.Lon, tt.stationLat, tt.stationLon))
	d.BearingToStation = CalculateBearing(latest.Lat, latest.Lon, tt.stationLat, tt.stationLon)

	// ── Short window: smoothed current values ──
	if len(shortValid) > 0 {
		d.GroundSpeedKts = mean(shortValid, func(s TrajectorySnapshot) float64 { return s.GS })
		d.VerticalRateFPM = medianFloat(shortValid, func(s TrajectorySnapshot) float64 { return s.BaroRate })
		d.TrackDeg = circularMean(shortValid, func(s TrajectorySnapshot) float64 { return s.Track })
	} else {
		d.GroundSpeedKts = latest.GS
		d.VerticalRateFPM = latest.BaroRate
		d.TrackDeg = latest.Track
	}

	// ── Short window: acceleration ──
	if len(shortValid) >= 2 {
		first := shortValid[0]
		last := shortValid[len(shortValid)-1]
		dt := last.Timestamp.Sub(first.Timestamp).Seconds()
		if dt > 0 {
			d.GSAccelKtsPerSec = (last.GS - first.GS) / dt
			d.VRAccelFPMPerSec = (last.BaroRate - first.BaroRate) / dt
			d.TrackRateDegPerSec = circularDiff(last.Track, first.Track) / dt
		}
	}

	// ── Medium window: altitude statistics ──
	window := mediumValid
	if len(window) == 0 {
		window = allValid
	}

	d.AltMean = mean(window, func(s TrajectorySnapshot) float64 { return s.AltBaro })
	d.AltMin, d.AltMax = minMax(window, func(s TrajectorySnapshot) float64 { return s.AltBaro })
	d.AltTrendFPM = olsSlope(window, func(s TrajectorySnapshot) float64 { return s.AltBaro }) * 60.0 // Convert ft/sec to fpm

	// ── Medium window: speed statistics ──
	d.GSMean = mean(window, func(s TrajectorySnapshot) float64 { return s.GS })
	d.GSMin, d.GSMax = minMax(window, func(s TrajectorySnapshot) float64 { return s.GS })
	d.GSTrendKtsPerSec = olsSlope(window, func(s TrajectorySnapshot) float64 { return s.GS })

	// ── Medium window: vertical rate statistics ──
	d.VRMean = mean(window, func(s TrajectorySnapshot) float64 { return s.BaroRate })
	d.VRStdDev = stdDev(window, func(s TrajectorySnapshot) float64 { return s.BaroRate })

	// ── Medium window: distance to station trend ──
	d.DistTrendNMPerSec = olsSlopeWithXY(window, func(s TrajectorySnapshot) (float64, float64) {
		return s.Timestamp.Sub(window[0].Timestamp).Seconds(),
			MetersToNM(Haversine(s.Lat, s.Lon, tt.stationLat, tt.stationLon))
	})

	// ── Boolean flags ──
	cfg := tt.config
	d.IsDescending = d.AltTrendFPM < cfg.DescentVRThresholdFPM && d.VRMean < 0
	d.IsClimbing = d.AltTrendFPM > cfg.ClimbVRThresholdFPM && d.VRMean > 0
	d.IsLevel = (d.AltMax - d.AltMin) < cfg.LevelAltBandFt
	d.IsDecelerating = d.GSTrendKtsPerSec < cfg.DecelerationThreshold
	d.IsAccelerating = d.GSTrendKtsPerSec > cfg.AccelerationThreshold
	d.IsTurning = math.Abs(d.TrackRateDegPerSec) > cfg.TurningRateThresholdDeg
	d.IsApproachingStation = d.DistTrendNMPerSec < -0.001 // Slightly negative threshold to avoid noise
}

// ─── Statistical Helpers ──────────────────────────────────────────────────────

// mean computes the arithmetic mean of a field extracted from snapshots.
func mean(snaps []TrajectorySnapshot, extract func(TrajectorySnapshot) float64) float64 {
	if len(snaps) == 0 {
		return 0
	}
	sum := 0.0
	for _, s := range snaps {
		sum += extract(s)
	}
	return sum / float64(len(snaps))
}

// minMax returns the minimum and maximum values of a field.
func minMax(snaps []TrajectorySnapshot, extract func(TrajectorySnapshot) float64) (float64, float64) {
	if len(snaps) == 0 {
		return 0, 0
	}
	mn := math.Inf(1)
	mx := math.Inf(-1)
	for _, s := range snaps {
		v := extract(s)
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}

// medianFloat computes the median of a field. Allocates a temporary slice for sorting.
func medianFloat(snaps []TrajectorySnapshot, extract func(TrajectorySnapshot) float64) float64 {
	n := len(snaps)
	if n == 0 {
		return 0
	}
	vals := make([]float64, n)
	for i, s := range snaps {
		vals[i] = extract(s)
	}
	sort.Float64s(vals)
	if n%2 == 0 {
		return (vals[n/2-1] + vals[n/2]) / 2
	}
	return vals[n/2]
}

// stdDev computes the sample standard deviation of a field.
func stdDev(snaps []TrajectorySnapshot, extract func(TrajectorySnapshot) float64) float64 {
	n := len(snaps)
	if n < 2 {
		return 0
	}
	m := mean(snaps, extract)
	sumSq := 0.0
	for _, s := range snaps {
		diff := extract(s) - m
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(n-1))
}

// olsSlope computes the OLS linear regression slope of a field against time.
// Time is measured in seconds from the first snapshot. Returns the slope in
// units-per-second (e.g., feet-per-second for altitude).
func olsSlope(snaps []TrajectorySnapshot, extract func(TrajectorySnapshot) float64) float64 {
	n := len(snaps)
	if n < 2 {
		return 0
	}
	t0 := snaps[0].Timestamp
	var sumT, sumY, sumTY, sumTT float64
	for _, s := range snaps {
		t := s.Timestamp.Sub(t0).Seconds()
		y := extract(s)
		sumT += t
		sumY += y
		sumTY += t * y
		sumTT += t * t
	}
	nf := float64(n)
	denom := nf*sumTT - sumT*sumT
	if math.Abs(denom) < 1e-12 {
		return 0 // All points at same time
	}
	return (nf*sumTY - sumT*sumY) / denom
}

// olsSlopeWithXY computes OLS slope with an explicit (x, y) extractor,
// used for distance-to-station trend where x = time offset, y = distance.
func olsSlopeWithXY(snaps []TrajectorySnapshot, extractXY func(TrajectorySnapshot) (float64, float64)) float64 {
	n := len(snaps)
	if n < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumXX float64
	for _, s := range snaps {
		x, y := extractXY(s)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	nf := float64(n)
	denom := nf*sumXX - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return 0
	}
	return (nf*sumXY - sumX*sumY) / denom
}

// olsR2 computes the R² (coefficient of determination) for the OLS linear
// regression of a field against time. Returns 0 if fewer than 3 points or no
// variance in the data. R² = 1 - SS_res/SS_tot.
func olsR2(snaps []TrajectorySnapshot, extract func(TrajectorySnapshot) float64) float64 {
	n := len(snaps)
	if n < 3 {
		return 0
	}
	t0 := snaps[0].Timestamp

	// Compute OLS coefficients (intercept + slope)
	var sumT, sumY, sumTY, sumTT float64
	for _, s := range snaps {
		t := s.Timestamp.Sub(t0).Seconds()
		y := extract(s)
		sumT += t
		sumY += y
		sumTY += t * y
		sumTT += t * t
	}
	nf := float64(n)
	denom := nf*sumTT - sumT*sumT
	if math.Abs(denom) < 1e-12 {
		return 0
	}
	slope := (nf*sumTY - sumT*sumY) / denom
	intercept := (sumY - slope*sumT) / nf

	// Compute R² = 1 - SS_res / SS_tot
	meanY := sumY / nf
	var ssRes, ssTot float64
	for _, s := range snaps {
		t := s.Timestamp.Sub(t0).Seconds()
		y := extract(s)
		predicted := intercept + slope*t
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}
	if ssTot < 1e-20 {
		return 0 // No variance
	}
	r2 := 1.0 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}
	return r2
}

// circularMean computes the mean of angles (degrees) handling 0°/360° wraparound.
func circularMean(snaps []TrajectorySnapshot, extract func(TrajectorySnapshot) float64) float64 {
	if len(snaps) == 0 {
		return 0
	}
	var sinSum, cosSum float64
	for _, s := range snaps {
		rad := extract(s) * math.Pi / 180.0
		sinSum += math.Sin(rad)
		cosSum += math.Cos(rad)
	}
	meanRad := math.Atan2(sinSum/float64(len(snaps)), cosSum/float64(len(snaps)))
	deg := meanRad * 180.0 / math.Pi
	return math.Mod(deg+360, 360)
}

// circularDiff returns the signed angular difference (a - b) in degrees,
// handling the 0°/360° wraparound. Result is in [-180, 180].
func circularDiff(a, b float64) float64 {
	diff := a - b
	for diff > 180 {
		diff -= 360
	}
	for diff < -180 {
		diff += 360
	}
	return diff
}
