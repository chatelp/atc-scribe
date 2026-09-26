package main

import (
	"strings"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/transcription/phraseology"
)

// adsbFleet lets the callsign matcher see what the receiver sees.
//
// This is the only place the transcription chain and the ADS-B service meet, and
// it is deliberately a translation rather than a shared type: the matcher needs
// four facts about an aircraft, and giving it the whole ADS-B model would tie the
// two packages together for the sake of fields it never reads.
type adsbFleet struct {
	service         *adsb.Service
	lastSeenMinutes int
}

// Fleet returns the aircraft that reported recently enough to plausibly be the
// one talking.
//
// The window matters. Measured against a shuffled control, a wider window buys a
// few more true matches and a great many more false ones, because every extra
// aircraft is another chance for four digits to coincide. One minute is what the
// published figures in docs-fr/18-nuit-du-15.md were measured at.
func (f *adsbFleet) Fleet() []phraseology.Aircraft {
	minutes := f.lastSeenMinutes
	if minutes <= 0 {
		minutes = 1
	}

	targets := f.service.GetAllAircraftWithLastSeenFilter(minutes)
	fleet := make([]phraseology.Aircraft, 0, len(targets))

	for _, target := range targets {
		callsign := strings.TrimSpace(target.Flight)
		if callsign == "" {
			// No callsign broadcast: nothing for a spoken callsign to match.
			continue
		}

		aircraft := phraseology.Aircraft{
			Callsign: callsign,
			Hex:      target.Hex,
		}
		if target.ADSB != nil {
			aircraft.AltitudeFt = target.ADSB.AltBaro.Float64()
			if target.ADSB.Lat != nil && target.ADSB.Lon != nil {
				aircraft.Lat, aircraft.Lon, aircraft.HasPosition = *target.ADSB.Lat, *target.ADSB.Lon, true
			}
		}
		if target.Phase != nil && len(target.Phase.Current) > 0 {
			aircraft.Phase = target.Phase.Current[0].Phase
		}

		fleet = append(fleet, aircraft)
	}

	return fleet
}
