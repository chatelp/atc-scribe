package adsb

import (
	"math"
	"time"
)

// ─── Trajectory Prediction ──────────────────────────────────────────────────
//
// Provides hindcast (backward extrapolation) and forecast (forward extrapolation)
// for aircraft trajectories. Both use the trajectory ring buffer and DerivedState
// for statistically grounded predictions.
//
// Hindcast: Extends the track backwards ~60 seconds before the first ADS-B
// observation, answering "where was this aircraft before we saw it?" Computed
// once when enough data has accumulated (≥5 valid points, R² ≥ 0.80), then
// locked in. Uses OLS linear regression on position components.
//
// Forecast: Extends the track forward 1 minute using a kinematic model
// with acceleration and turn rate from DerivedState; from a single sample
// (ForecastFromTarget) when the tracker has no forecast. It replaced the
// naive constant-heading/constant-speed PredictFuturePositions(), removed.

const (
	// Hindcast: 6 points × 10s = 60 seconds backward
	hindcastSteps     = 6
	hindcastStepSec   = 10.0
	hindcastMinPoints = 5    // minimum valid points before attempting hindcast
	hindcastMaxPoints = 10   // max points to use for hindcast regression
	hindcastMinR2     = 0.80 // minimum R² confidence for hindcast
	hindcastLockAfter = 30.0 // lock hindcast after this many seconds of tracking
	hindcastDecayBase = 0.95 // per-step confidence decay for hindcast points

	// Forecast: 12 points × 5s = 1 minute forward
	forecastSteps     = 12
	forecastStepSec   = 5.0
	forecastDecayBase = 0.99 // per-step confidence decay for forecast points

	turnPenaltyMaxRate = 3.0         // deg/sec at which turn penalty reaches 0
	kmPerNM            = 1.852       // kilometers per nautical mile
	degPerKmLat        = 1.0 / 111.0 // approximate degrees latitude per km
)

// PredictionPoint represents a single predicted position with confidence.
type PredictionPoint struct {
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Altitude   float64   `json:"altitude"`
	Heading    float64   `json:"heading"`
	Speed      float64   `json:"speed"`      // ground speed (knots)
	Confidence float64   `json:"confidence"` // 0.0 - 1.0
	Timestamp  time.Time `json:"timestamp"`
}

// TrajectoryPrediction holds computed hindcast and forecast for one aircraft.
type TrajectoryPrediction struct {
	Hindcast       []PredictionPoint // Past predictions (oldest first, before first obs)
	Forecast       []PredictionPoint // Future predictions (nearest first, after latest obs)
	HindcastLocked bool              // Once locked, don't recompute
	ComputedAt     time.Time
}

// ─── Hindcast (backward extrapolation) ──────────────────────────────────────

// computeHindcast extrapolates the trajectory backward from the earliest
// observation using OLS linear regression on position components.
//
// The algorithm:
// 1. Collect earliest valid snapshots (up to hindcastMaxPoints)
// 2. Fit OLS regression on lat(t), lon(t), alt(t)
// 3. Compute R² for lat and lon — must both be ≥ hindcastMinR2
// 4. Apply turn penalty and data density factor to get composite confidence
// 5. If confidence ≥ 0.80, extrapolate backward 6 steps of 10 seconds each
// 6. Lock the hindcast once computed or after hindcastLockAfter seconds
func computeHindcast(at *AircraftTrajectory, derived *DerivedState) {
	pred := &at.Prediction
	if pred.HindcastLocked {
		return
	}

	// Collect valid snapshots oldest-first
	var validSnaps []TrajectorySnapshot
	at.ForEachSnapshot(func(snap *TrajectorySnapshot) {
		if snap.Valid {
			validSnaps = append(validSnaps, *snap)
		}
	})

	if len(validSnaps) < hindcastMinPoints {
		return
	}

	// Check if we should lock (too much time has passed)
	earliest := validSnaps[0]
	latest := validSnaps[len(validSnaps)-1]
	trackingDuration := latest.Timestamp.Sub(earliest.Timestamp).Seconds()

	// Use earliest points for regression (they're closest to the extrapolation target)
	n := len(validSnaps)
	if n > hindcastMaxPoints {
		n = hindcastMaxPoints
	}
	regSnaps := validSnaps[:n]

	// OLS regression on lat, lon, alt vs time
	r2Lat := olsR2(regSnaps, func(s TrajectorySnapshot) float64 { return s.Lat })
	r2Lon := olsR2(regSnaps, func(s TrajectorySnapshot) float64 { return s.Lon })

	slopeLat := olsSlope(regSnaps, func(s TrajectorySnapshot) float64 { return s.Lat })
	slopeLon := olsSlope(regSnaps, func(s TrajectorySnapshot) float64 { return s.Lon })
	slopeAlt := olsSlope(regSnaps, func(s TrajectorySnapshot) float64 { return s.AltBaro })

	// Compute heading from regression slopes for the predicted points
	headingRad := math.Atan2(slopeLon, slopeLat)
	heading := math.Mod(headingRad*180.0/math.Pi+360, 360)

	// Speed from regression slopes (degrees/sec → knots)
	// lat slope is in deg/sec, lon slope is in deg/sec
	latKmPerSec := slopeLat * 111.0
	lonKmPerSec := slopeLon * 111.0 * math.Cos(earliest.Lat*math.Pi/180.0)
	speedKmPerSec := math.Sqrt(latKmPerSec*latKmPerSec + lonKmPerSec*lonKmPerSec)
	speedKts := speedKmPerSec * 3600.0 / kmPerNM

	// Composite confidence
	r2Min := math.Min(r2Lat, r2Lon)

	// Turn penalty: straight flight = 1.0, turning ≥3°/s = 0.0
	turnRate := 0.0
	if derived != nil {
		turnRate = math.Abs(derived.TrackRateDegPerSec)
	}
	turnPenalty := math.Max(0, 1.0-turnRate/turnPenaltyMaxRate)

	// Data density factor: 5 points = 0.5, 10+ = 1.0
	dataDensity := math.Min(1.0, float64(len(regSnaps))/float64(hindcastMaxPoints))

	confidence := r2Min * turnPenalty * dataDensity

	// Lock after threshold time regardless of confidence
	if trackingDuration >= hindcastLockAfter {
		pred.HindcastLocked = true
		if confidence < hindcastMinR2 {
			// Not confident enough — lock with no hindcast
			pred.Hindcast = nil
			return
		}
	}

	if confidence < hindcastMinR2 {
		return // Not ready yet, try again next cycle
	}

	// Generate hindcast points going backward from earliest observation
	points := make([]PredictionPoint, hindcastSteps)
	for i := 0; i < hindcastSteps; i++ {
		stepsBack := float64(hindcastSteps - i) // 6, 5, 4, 3, 2, 1 (oldest first)
		dt := -stepsBack * hindcastStepSec      // negative time offset from earliest

		lat := earliest.Lat + slopeLat*dt
		lon := earliest.Lon + slopeLon*dt
		alt := earliest.AltBaro + slopeAlt*dt
		if alt < 0 {
			alt = 0
		}

		pointConf := confidence * math.Pow(hindcastDecayBase, stepsBack)

		points[i] = PredictionPoint{
			Lat:        lat,
			Lon:        lon,
			Altitude:   alt,
			Heading:    heading,
			Speed:      speedKts,
			Confidence: pointConf,
			Timestamp:  earliest.Timestamp.Add(time.Duration(dt * float64(time.Second))),
		}
	}

	pred.Hindcast = points
	pred.HindcastLocked = true
}

// ─── Forecast (forward extrapolation) ───────────────────────────────────────

// computeForecast extrapolates the trajectory forward from the latest
// observation using a kinematic model with acceleration and turn rate.
//
// Uses DerivedState trends for physics-based prediction:
// - Speed changes with GSAccelKtsPerSec (clamped to reasonable bounds)
// - Heading changes with TrackRateDegPerSec (curved paths)
// - Altitude changes with AltTrendFPM (OLS-derived, more stable than instantaneous VR)
//
// Confidence decays per step, faster for turning/accelerating aircraft.
func computeForecast(at *AircraftTrajectory, derived *DerivedState) {
	pred := &at.Prediction
	latest := at.Latest()
	if latest == nil || !latest.Valid {
		pred.Forecast = nil
		return
	}

	if derived == nil || derived.ValidPointCount < 2 {
		// Not enough data for trajectory-based forecast
		pred.Forecast = nil
		return
	}

	// Starting state from DerivedState (smoothed) + latest position. The
	// forecast starts where and when the position was decoded, not at the
	// poll that repeated it: the live prediction interpolates from that fix.
	pred.Forecast = forecastPoints(forecastStart{
		lat: latest.Lat, lon: latest.Lon, alt: derived.AltMean,
		speed: derived.GroundSpeedKts, heading: derived.TrackDeg,
		gsAccel: derived.GSAccelKtsPerSec, trackRate: derived.TrackRateDegPerSec,
		altRate: derived.AltTrendFPM / 60.0, // fpm to ft/sec
		at:      latest.PositionTime(),
	})
	pred.ComputedAt = latest.PositionTime()
}

// ForecastFromTarget is the forecast of an aircraft the trajectory tracker
// cannot forecast -- flight phases turned off (no tracker), a first poll, an
// aircraft read back from the database: the same kinematic model started
// from its one sample, speed, track and vertical rate held, the position
// dated at. Nil without a position, an altitude, a speed and a heading, as
// before (upstream's PredictFuturePositions, which this replaces).
func ForecastFromTarget(t *ADSBTarget, at time.Time) []PredictionPoint {
	if t == nil || !t.HasPosition() || t.AltBaro.Float64() == 0 {
		return nil
	}
	lat, lon, _ := t.Position()
	heading := NumberOrZero(t.Track)
	if heading == 0 {
		heading = NumberOrZero(t.TrueHeading)
	}
	if heading == 0 {
		heading = NumberOrZero(t.MagHeading)
	}
	speed := NumberOrZero(t.GS)
	if speed == 0 {
		speed = NumberOrZero(t.TAS)
	}
	if heading == 0 || speed == 0 {
		return nil
	}
	vr := NumberOrZero(t.BaroRate)
	if vr == 0 {
		vr = NumberOrZero(t.GeomRate)
	}
	return forecastPoints(forecastStart{
		lat: lat, lon: lon, alt: t.AltBaro.Float64(), speed: speed, heading: heading,
		altRate: vr / 60.0, at: at,
	})
}

// forecastStart is where a forecast starts: a position, its time, and the
// kinematics carried forward.
type forecastStart struct {
	lat, lon, alt      float64 // degrees, feet
	speed, heading     float64 // knots, degrees
	gsAccel, trackRate float64 // knots/sec, degrees/sec
	altRate            float64 // feet/sec
	at                 time.Time
}

// forecastPoints extrapolates forward with a kinematic model:
// - Speed changes with gsAccel (clamped to reasonable bounds)
// - Heading changes with trackRate (curved paths)
// - Altitude changes with altRate
//
// Confidence decays per step, faster for turning/accelerating aircraft.
func forecastPoints(st forecastStart) []PredictionPoint {
	curLat, curLon := st.lat, st.lon
	curAlt, curSpeed, curHeading := st.alt, st.speed, st.heading
	gsAccel, trackRate, altRate := st.gsAccel, st.trackRate, st.altRate

	// Confidence factors
	turnFactor := math.Max(0.3, 1.0-math.Abs(trackRate)*0.15)
	accelFactor := math.Max(0.5, 1.0-math.Abs(gsAccel)*0.1)

	points := make([]PredictionPoint, forecastSteps)
	now := st.at

	for i := 0; i < forecastSteps; i++ {
		step := float64(i + 1)
		dt := step * forecastStepSec // seconds from now

		// Speed with acceleration, clamped
		speed := curSpeed + gsAccel*dt
		if speed < 0 {
			speed = 0
		}
		maxSpeed := curSpeed * 2.0
		if maxSpeed < 100 {
			maxSpeed = 100
		}
		if speed > maxSpeed {
			speed = maxSpeed
		}

		// Heading with turn rate
		heading := curHeading + trackRate*dt
		heading = math.Mod(heading+360, 360)

		// Distance traveled this step (from previous position, not from start)
		// Use average speed over this step interval (trapezoidal integration)
		prevDt := (step - 1) * forecastStepSec
		prevSpeed := curSpeed + gsAccel*prevDt
		if prevSpeed < 0 {
			prevSpeed = 0
		}
		avgSpeed := (prevSpeed + speed) / 2.0
		distKm := avgSpeed * kmPerNM / 3600.0 * forecastStepSec

		// Average heading during this step interval for position integration
		prevHeading := curHeading + trackRate*prevDt
		avgHeadingRad := (prevHeading + heading) / 2.0 * math.Pi / 180.0

		// Position update using average heading over the step
		cosLat := math.Cos(curLat * math.Pi / 180.0)
		if cosLat < 0.01 {
			cosLat = 0.01
		}
		latChange := distKm * math.Cos(avgHeadingRad) * degPerKmLat
		lonChange := distKm * math.Sin(avgHeadingRad) * degPerKmLat / cosLat

		// Cumulative position from start
		// For proper integration, sum incremental changes
		if i == 0 {
			curLat += latChange
			curLon += lonChange
		} else {
			// Update from previous predicted position
			curLat = points[i-1].Lat + latChange
			curLon = points[i-1].Lon + lonChange
		}

		// Altitude
		alt := curAlt + altRate*dt
		if alt < 0 {
			alt = 0
		}

		// Confidence
		conf := math.Pow(turnFactor, step) * math.Pow(accelFactor, step) * math.Pow(forecastDecayBase, step)

		points[i] = PredictionPoint{
			Lat:        curLat,
			Lon:        curLon,
			Altitude:   alt,
			Heading:    heading,
			Speed:      speed,
			Confidence: conf,
			Timestamp:  now.Add(time.Duration(dt * float64(time.Second))),
		}
	}

	return points
}

// ─── Conversion Helpers ─────────────────────────────────────────────────────

// PredictionPointsToPositions converts prediction points to the Position type
// used by the API. Confidence is not preserved (Position doesn't have it).
func PredictionPointsToPositions(points []PredictionPoint) []Position {
	if len(points) == 0 {
		return nil
	}
	positions := make([]Position, len(points))
	for i, p := range points {
		positions[i] = Position{
			Lat:         NumberPtr(p.Lat),
			Lon:         NumberPtr(p.Lon),
			Altitude:    NumberPtr(p.Altitude),
			SpeedGS:     NumberPtr(p.Speed),
			SpeedTrue:   NumberPtr(p.Speed),
			TrueHeading: NumberPtr(p.Heading),
			MagHeading:  NumberPtr(p.Heading),
			Timestamp:   p.Timestamp,
		}
	}
	return positions
}
