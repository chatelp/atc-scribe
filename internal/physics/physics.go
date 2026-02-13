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

func HeadingToVector(headingDeg float64, magnitude float64) Vector2D {
	rad := (90 - headingDeg) * math.Pi / 180
	return Vector2D{X: magnitude * math.Cos(rad), Y: magnitude * math.Sin(rad)}
}

func AltitudeToPressure(altFt float64) float64 {
	altM := altFt * 0.3048
	if altM < 0 {
		altM = 0
	}
	if altM <= TropopauseAltM {
		exponent := G / (R * L)
		base := 1 - (L*altM)/T0
		return P0 * math.Pow(base, exponent)
	}
	relAlt := altM - TropopauseAltM
	exponent := -(G * relAlt) / (R * StratosphereTempK)
	return TropopausePress * math.Exp(exponent)
}

func CalculateSoundSpeed(tempK float64) float64 {
	if tempK <= 0 {
		return 0
	}
	return math.Sqrt(Gamma * R * tempK)
}

func CalculateMach(tasKnots float64, tempCelsius float64) float64 {
	tempK := tempCelsius + ZeroCelsius
	a := CalculateSoundSpeed(tempK)
	if a == 0 {
		return 0
	}
	tasMs := tasKnots * KnotsToMs
	return tasMs / a
}

func CalculateTAT(oatCelsius float64, mach float64) float64 {
	oatK := oatCelsius + ZeroCelsius
	tatK := oatK * (1.0 + 0.2*mach*mach)
	return tatK - ZeroCelsius
}

func CalculateTASFromMach(mach float64, tempCelsius float64) float64 {
	tempK := tempCelsius + ZeroCelsius
	a := CalculateSoundSpeed(tempK)
	return mach * a * MsToKnots
}

func CalculateCAS(tasKnots float64, pressAltFt float64, tempCelsius float64) float64 {
	pHPa := AltitudeToPressure(pressAltFt)
	pPa := pHPa * 100.0
	p0Pa := P0 * 100.0
	mach := CalculateMach(tasKnots, tempCelsius)
	qc := pPa * (math.Pow(1+0.2*mach*mach, 3.5) - 1)
	a0 := CalculateSoundSpeed(T0) * MsToKnots
	term := (qc / p0Pa) + 1
	if term < 0 {
		return 0
	}
	return a0 * math.Sqrt(5*(math.Pow(term, 1/3.5)-1))
}

func CalculateDensityAltitude(pressureAltFt float64, tempCelsius float64) float64 {
	isaTempK := T0 - (L * (pressureAltFt * 0.3048))
	if pressureAltFt > TropopauseAltFt {
		isaTempK = StratosphereTempK
	}
	isaTempC := isaTempK - ZeroCelsius
	return pressureAltFt + 120*(tempCelsius-isaTempC)
}

func SolveWindTriangle(gsKnots float64, trackDeg float64, windU_Ms float64, windV_Ms float64) (float64, float64) {
	windU_Kts := windU_Ms * MsToKnots
	windV_Kts := windV_Ms * MsToKnots

	groundVec := HeadingToVector(trackDeg, gsKnots)
	windVec := Vector2D{X: windU_Kts, Y: windV_Kts}

	airVecX := groundVec.X - windVec.X
	airVecY := groundVec.Y - windVec.Y

	tas := math.Sqrt(airVecX*airVecX + airVecY*airVecY)
	rad := math.Atan2(airVecY, airVecX)
	heading := 90 - (rad * 180 / math.Pi)
	heading = NormalizeHeading(heading)

	return tas, heading
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
