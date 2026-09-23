package main

import (
	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/reference"
	"github.com/yegors/co-atc/internal/weather"
	"github.com/yegors/co-atc/pkg/logger"
)

// referenceAirport wires the reference airport -- the one approach, departure,
// takeoff and landing are judged against, and whose weather is fetched -- to the
// three services that each hold a piece of it.
//
// It exists because a receiver does not always sit on the airport worth
// watching. Upstream assumed it did and measured every phase from the receiver;
// see adsb.PhaseReference.
type referenceAirport struct {
	ref     *reference.Service
	adsb    *adsb.Service
	weather *weather.Service // set once the weather service exists
	log     *logger.Logger
}

// validate has no side effect: a settings change is refused before any part of
// it is applied.
func (r *referenceAirport) validate(code string) error {
	_, err := r.ref.CheckHomeAirport(code)
	return err
}

// apply switches the three services to code. It cannot fail once validate has
// passed, short of the reference data changing in between, which it does not
// while the server runs.
func (r *referenceAirport) apply(code string) {
	airport, _, err := r.ref.SetHomeAirport(code)
	if err != nil {
		r.log.Error("Reference airport could not be applied",
			logger.String("code", code), logger.Error(err))
		return
	}
	r.adsb.SetPhaseReference(adsb.PhaseReference{
		Airport: airport.Ident,
		Lat:     airport.Latitude,
		Lon:     airport.Longitude,
		Runways: r.ref.GetHomeRunwayData(),
	})
	if r.weather != nil {
		r.weather.SetAirport(airport.Ident)
	}
	r.log.Info("Reference airport in force",
		logger.String("code", airport.Ident), logger.String("name", airport.Name))
}

// start applies the airport saved from the panel, or else the configured one.
// A saved airport can become unusable -- the display range or the data changed
// -- and falling back is said in the log rather than done silently. It returns
// the airport actually in force.
func (r *referenceAirport) start(saved, configured string) string {
	for _, code := range []string{saved, configured} {
		if code == "" {
			continue
		}
		if err := r.validate(code); err != nil {
			r.log.Warn("Reference airport unusable",
				logger.String("code", code), logger.Error(err))
			continue
		}
		r.apply(code)
		return code
	}
	// Neither is usable: phases are judged from the receiver, which is exactly
	// upstream's behaviour.
	r.adsb.SetPhaseReference(adsb.PhaseReference{
		Airport: configured,
		Runways: r.ref.GetHomeRunwayData(),
	})
	r.log.Warn("No usable reference airport; phases are judged from the receiver's position")
	return configured
}
