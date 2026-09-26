package phraseology

import "math"

// Sector is the part of the sky a frequency can be talking to: a radius around
// the airport it serves, below a ceiling. An approach frequency over Paris hears
// its transmissions compared with a hundred aircraft of which a quarter are in
// its sector; the rest can only coincide. Measured on 24/09: cutting the sky to
// 60 NM and 20,000 ft around Roissy halved the chance matches and gained true
// ones (docs-fr/05-decisions.md, Q46).
type Sector struct {
	Lat, Lon float64 // the airport
	RadiusNM float64
	MaxAltFt float64 // 0: no ceiling
}

// Keep reports whether an aircraft may be in the sector. What is not known is
// kept: an aircraft without a position, or without an altitude, is never
// excluded on a guess.
func (s Sector) Keep(ac Aircraft) bool {
	if s.MaxAltFt > 0 && ac.AltitudeFt > s.MaxAltFt {
		return false
	}
	if !ac.HasPosition || s.RadiusNM <= 0 {
		return true
	}
	return distanceNM(s.Lat, s.Lon, ac.Lat, ac.Lon) <= s.RadiusNM
}

// Within returns the aircraft of the fleet the sector keeps.
func (s Sector) Within(fleet []Aircraft) []Aircraft {
	out := make([]Aircraft, 0, len(fleet))
	for _, ac := range fleet {
		if s.Keep(ac) {
			out = append(out, ac)
		}
	}
	return out
}

func distanceNM(lat1, lon1, lat2, lon2 float64) float64 {
	r := math.Pi / 180
	dLat, dLon := (lat2-lat1)*r, (lon2-lon1)*r
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*r)*math.Cos(lat2*r)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 3440.1 * 2 * math.Asin(math.Sqrt(a))
}
