package adsb

import (
	"math"

	"github.com/yegors/co-atc/internal/physics"
)

func computeATCDerivedMetrics(target *ADSBTarget, distanceNm *float64) *ATCDerivedMetrics {
	if target == nil {
		return nil
	}

	derived := &ATCDerivedMetrics{}

	isMoving := target.GS > 25 || target.TAS > 25

	track, hasTrack := pickHeadingLikeValue(target.Track, isMoving)
	referenceHeading, headingSource, hasHeading := pickReferenceHeading(target, isMoving)

	if headingSource != "" {
		derived.HeadingSource = headingSource
	}

	if hasTrack && hasHeading {
		drift := physics.SignedAngleDiffDeg(track, referenceHeading)
		derived.TrackHeadingErrorDeg = floatPtr(drift)
	}

	if hasTrack && target.WD >= 0 && target.WS > 0 {
		headwind, crosswind := physics.WindComponents(track, target.WD, target.WS)
		derived.HeadTailwindKt = floatPtr(headwind)
		derived.CrosswindKt = floatPtr(crosswind)
	}

	verticalRate, hasVerticalRate := pickVerticalRate(target)
	if hasVerticalRate && target.GS > 1 {
		fpa := physics.FlightPathAngleDeg(verticalRate, target.GS)
		gradient := physics.ClimbGradientFtPerNm(verticalRate, target.GS)
		derived.FlightPathAngleDeg = floatPtr(fpa)
		derived.ClimbGradientFtNm = floatPtr(gradient)
	}

	if math.Abs(target.TrackRate) > 0 {
		derived.TurnRateDegSec = floatPtr(target.TrackRate)
	}

	if distanceNm != nil && *distanceNm >= 0 && target.GS > 30 {
		etaSec := (*distanceNm / target.GS) * 3600.0
		derived.ETAStationSec = floatPtr(etaSec)
	}

	if isATCDerivedEmpty(derived) {
		return nil
	}

	return derived
}

func AttachATCDerivedMetrics(aircraft *Aircraft) {
	if aircraft == nil || aircraft.ADSB == nil {
		return
	}
	aircraft.ADSB.ATCDerived = computeATCDerivedMetrics(aircraft.ADSB, aircraft.Distance)
}

func pickReferenceHeading(target *ADSBTarget, moving bool) (float64, string, bool) {
	if v, ok := pickHeadingLikeValue(target.TrueHeading, moving); ok {
		return v, "TRUE", true
	}
	if v, ok := pickHeadingLikeValue(target.MagHeading, moving); ok {
		return v, "MAG", true
	}
	if v, ok := pickHeadingLikeValue(target.Track, moving); ok {
		return v, "TRACK", true
	}
	return 0, "", false
}

func pickHeadingLikeValue(value float64, moving bool) (float64, bool) {
	if value > 0 && value < 360 {
		return value, true
	}
	if value == 0 && moving {
		return 0, true
	}
	return 0, false
}

func pickVerticalRate(target *ADSBTarget) (float64, bool) {
	if target.BaroRate != 0 {
		return target.BaroRate, true
	}
	if target.GeomRate != 0 {
		return target.GeomRate, true
	}
	return 0, false
}

func isATCDerivedEmpty(v *ATCDerivedMetrics) bool {
	if v == nil {
		return true
	}
	return v.HeadingSource == "" &&
		v.TrackHeadingErrorDeg == nil &&
		v.HeadTailwindKt == nil &&
		v.CrosswindKt == nil &&
		v.FlightPathAngleDeg == nil &&
		v.ClimbGradientFtNm == nil &&
		v.TurnRateDegSec == nil &&
		v.ETAStationSec == nil
}

func atcDerivedEqual(a, b *ATCDerivedMetrics) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.HeadingSource == b.HeadingSource &&
		floatPtrEqual(a.TrackHeadingErrorDeg, b.TrackHeadingErrorDeg) &&
		floatPtrEqual(a.HeadTailwindKt, b.HeadTailwindKt) &&
		floatPtrEqual(a.CrosswindKt, b.CrosswindKt) &&
		floatPtrEqual(a.FlightPathAngleDeg, b.FlightPathAngleDeg) &&
		floatPtrEqual(a.ClimbGradientFtNm, b.ClimbGradientFtNm) &&
		floatPtrEqual(a.TurnRateDegSec, b.TurnRateDegSec) &&
		floatPtrEqual(a.ETAStationSec, b.ETAStationSec)
}

func floatPtr(v float64) *float64 {
	out := v
	return &out
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
