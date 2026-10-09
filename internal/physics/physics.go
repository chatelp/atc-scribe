package physics

// copied from @stignarnia

import "math"

const (
	R           = 287.058
	Gamma       = 1.4
	G           = 9.80665
	T0          = 288.15
	P0          = 1013.25
	L           = 0.0065
	ZeroCelsius = 273.15
	KnotsToMs   = 0.514444
	MsToKnots   = 1.94384

	TropopauseAltM    = 11000.0
	TropopauseAltFt   = 36089.2
	StratosphereTempK = 216.65
	TropopausePress   = 226.32
)

type Vector2D struct {
	X float64
	Y float64
}

func NormalizeHeading(deg float64) float64 {
	v := math.Mod(deg, 360)
	if v < 0 {
		v += 360
	}
	return v
}

func SignedAngleDiffDeg(a, b float64) float64 {
	d := NormalizeHeading(a) - NormalizeHeading(b)
	for d > 180 {
		d -= 360
	}
	for d <= -180 {
		d += 360
	}
	return d
}

func WindComponents(trackDeg float64, windFromDeg float64, windSpeedKnots float64) (float64, float64) {
	rel := SignedAngleDiffDeg(windFromDeg, trackDeg)
	rad := rel * math.Pi / 180.0
	headwind := windSpeedKnots * math.Cos(rad)
	crosswind := windSpeedKnots * math.Sin(rad)
	return headwind, crosswind
}

func FlightPathAngleDeg(verticalRateFpm float64, groundSpeedKnots float64) float64 {
	if groundSpeedKnots <= 0 {
		return 0
	}
	gsFpm := groundSpeedKnots * 101.2686
	return math.Atan2(verticalRateFpm, gsFpm) * 180 / math.Pi
}

func ClimbGradientFtPerNm(verticalRateFpm float64, groundSpeedKnots float64) float64 {
	if groundSpeedKnots <= 0 {
		return 0
	}
	return (verticalRateFpm * 60.0) / groundSpeedKnots
}
