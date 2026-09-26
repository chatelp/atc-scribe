package main

import (
	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/reference"
	"github.com/yegors/co-atc/internal/weather"
	"github.com/yegors/co-atc/pkg/logger"
)

// referenceAirport wires the reference airports -- the ones approach, departure,
// takeoff and landing are judged against -- to the three services that each
// hold a piece of them. The first is the principal: its weather is fetched, and
// it is the airport of the one-airport API fields. The others are followed
// besides it; each aircraft is judged against the one whose runway axis it
// flies (adsb.airportFor).
//
// It exists because a receiver does not always sit on the airport worth
// watching, nor on a single one. Upstream assumed it did and measured every
// phase from the receiver; see adsb.PhaseReference.
type referenceAirport struct {
	ref     *reference.Service
	adsb    *adsb.Service
	weather *weather.Service // set once the weather service exists
	log     *logger.Logger
}

// validate has no side effect: a settings change is refused before any part of
// it is applied.
func (r *referenceAirport) validate(codes []string) error {
	for _, code := range codes {
		if _, err := r.ref.CheckHomeAirport(code); err != nil {
			return err
		}
	}
	return nil
}

// apply switches the three services to codes, the principal first. It cannot
// fail once validate has passed, short of the reference data changing in
// between, which it does not while the server runs.
func (r *referenceAirport) apply(codes []string) {
	if len(codes) == 0 {
		return
	}
	airport, _, err := r.ref.SetHomeAirport(codes[0])
	if err != nil {
		r.log.Error("Reference airport could not be applied",
			logger.String("code", codes[0]), logger.Error(err))
		return
	}
	refs := []adsb.PhaseReference{{
		Airport: airport.Ident,
		Lat:     airport.Latitude,
		Lon:     airport.Longitude,
		Runways: r.ref.GetHomeRunwayData(),
	}}
	for _, code := range codes[1:] {
		other, runways, _, err := r.ref.AirportRunways(code)
		if err != nil {
			r.log.Error("Followed airport could not be applied",
				logger.String("code", code), logger.Error(err))
			continue
		}
		refs = append(refs, adsb.PhaseReference{
			Airport: other.Ident, Lat: other.Latitude, Lon: other.Longitude, Runways: runways,
		})
	}
	r.adsb.SetPhaseReferences(refs)
	if r.weather != nil {
		r.weather.SetAirport(airport.Ident)
	}
	for i, ref := range refs {
		r.log.Info("Reference airport in force",
			logger.String("code", ref.Airport), logger.Bool("principal", i == 0))
	}
}

// start applies the airports saved from the panel, or else the configured one.
// A saved airport can become unusable -- the display range or the data changed
// -- and falling back is said in the log rather than done silently: an unusable
// principal falls back to the configured airport, an unusable further airport
// is left out. It returns the airports actually in force, the principal first.
func (r *referenceAirport) start(saved []string, configured string) []string {
	var principal string
	for _, code := range []string{first(saved), configured} {
		if code == "" {
			continue
		}
		if err := r.validate([]string{code}); err != nil {
			r.log.Warn("Reference airport unusable",
				logger.String("code", code), logger.Error(err))
			continue
		}
		principal = code
		break
	}
	if principal == "" {
		// None is usable: phases are judged from the receiver, which is exactly
		// upstream's behaviour.
		r.adsb.SetPhaseReference(adsb.PhaseReference{
			Airport: configured,
			Runways: r.ref.GetHomeRunwayData(),
		})
		r.log.Warn("No usable reference airport; phases are judged from the receiver's position")
		return []string{configured}
	}
	codes := []string{principal}
	if len(saved) > 0 && saved[0] == principal {
		for _, code := range saved[1:] {
			if err := r.validate([]string{code}); err != nil {
				r.log.Warn("Followed airport unusable, left out",
					logger.String("code", code), logger.Error(err))
				continue
			}
			codes = append(codes, code)
		}
	}
	r.apply(codes)
	return codes
}

func first(codes []string) string {
	if len(codes) == 0 {
		return ""
	}
	return codes[0]
}
