package reference

import "math"

const (
	earthRadiusMeters = 6371000.0
	earthRadiusNM     = 3440.065 // 6371 km / 1.852 km/nm
	metersPerNM       = 1852.0
	degToRad          = math.Pi / 180.0
	radToDeg          = 180.0 / math.Pi
)

// haversineNM calculates the distance in nautical miles between two lat/lon points.
func haversineNM(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * degToRad
	lon1Rad := lon1 * degToRad
	lat2Rad := lat2 * degToRad
	lon2Rad := lon2 * degToRad

	dlon := lon2Rad - lon1Rad
	dlat := lat2Rad - lat1Rad

	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(dlon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return (earthRadiusMeters * c) / metersPerNM
}

// calculateBearing calculates the initial bearing from point 1 to point 2 in degrees.
func calculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	lat1r := lat1 * degToRad
	lon1r := lon1 * degToRad
	lat2r := lat2 * degToRad
	lon2r := lon2 * degToRad

	y := math.Sin(lon2r-lon1r) * math.Cos(lat2r)
	x := math.Cos(lat1r)*math.Sin(lat2r) - math.Sin(lat1r)*math.Cos(lat2r)*math.Cos(lon2r-lon1r)
	bearing := math.Atan2(y, x) * radToDeg

	return math.Mod(math.Mod(bearing, 360)+360, 360)
}

// calculateDestinationPoint calculates a destination point given start, bearing (degrees), and distance (NM).
func calculateDestinationPoint(lat, lon, bearing, distanceNM float64) (float64, float64) {
	latr := lat * degToRad
	lonr := lon * degToRad
	bearingr := bearing * degToRad

	distRatio := distanceNM / earthRadiusNM
	lat2 := math.Asin(math.Sin(latr)*math.Cos(distRatio) + math.Cos(latr)*math.Sin(distRatio)*math.Cos(bearingr))
	lon2 := lonr + math.Atan2(
		math.Sin(bearingr)*math.Sin(distRatio)*math.Cos(latr),
		math.Cos(distRatio)-math.Sin(latr)*math.Sin(lat2),
	)

	return lat2 * radToDeg, lon2 * radToDeg
}
