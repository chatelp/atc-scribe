package templating

import (
	"fmt"
	"strings"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/weather"
)

// FormatAircraftData formats aircraft data for template rendering
// Uses the same format as ATC chat for consistency
func FormatAircraftData(aircraft []*adsb.Aircraft, airport AirportInfo) string {
	if len(aircraft) == 0 {
		return "No aircraft currently in the airspace."
	}

	// Separate aircraft by ground status
	var airborne []*adsb.Aircraft
	var onGround []*adsb.Aircraft

	for _, ac := range aircraft {
		if ac.OnGround {
			onGround = append(onGround, ac)
		} else {
			airborne = append(airborne, ac)
		}
	}

	var builder strings.Builder

	// Airborne aircraft section
	builder.WriteString(fmt.Sprintf("AIRBORNE (%d aircraft):\n", len(airborne)))
	if len(airborne) == 0 {
		builder.WriteString("No airborne aircraft\n")
	} else {
		for _, ac := range airborne {
			builder.WriteString(formatAirborneAircraft(ac, airport))
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n")

	// Ground aircraft section
	builder.WriteString(fmt.Sprintf("ON GROUND (%d aircraft):\n", len(onGround)))
	if len(onGround) == 0 {
		builder.WriteString("No ground aircraft\n")
	} else {
		for _, ac := range onGround {
			builder.WriteString(formatGroundAircraft(ac, airport))
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// formatAirborneAircraft formats a single airborne aircraft for display
func formatAirborneAircraft(ac *adsb.Aircraft, airport AirportInfo) string {
	var builder strings.Builder

	// Basic info - callsign and operator
	callsign := ac.Flight
	if callsign == "" {
		callsign = "Unknown"
	}

	builder.WriteString(fmt.Sprintf("%s", callsign))

	// Operator from ADSB data
	if ac.Airline != "" {
		builder.WriteString(fmt.Sprintf(" (%s)", ac.Airline))
	}

	builder.WriteString(" | ")

	// Aircraft type
	if ac.ADSB != nil && ac.ADSB.AircraftType != "" {
		builder.WriteString(fmt.Sprintf("Type: %s | ", ac.ADSB.AircraftType))
	}

	// Wake category
	if ac.ADSB != nil && ac.ADSB.Category != "" {
		builder.WriteString(fmt.Sprintf("Wake Category: %s | ", ac.ADSB.Category))
	}

	// Flight parameters
	if ac.ADSB != nil {
		builder.WriteString("Flight params: ")

		// Use magnetic heading if available (0-360° are all valid), fallback to track
		if ac.ADSB.MagHeading >= 0 && ac.ADSB.MagHeading <= 360 {
			builder.WriteString(fmt.Sprintf("HDG: %.0f", ac.ADSB.MagHeading))
		} else if ac.ADSB.Track != 0 {
			builder.WriteString(fmt.Sprintf("HDG: %.0f", ac.ADSB.Track))
		}

		if ac.ADSB.TAS != 0 {
			builder.WriteString(fmt.Sprintf(", TAS: %.0f kts", ac.ADSB.TAS))
		}

		if ac.ADSB.GS != 0 {
			builder.WriteString(fmt.Sprintf(", GS: %.0f kts", ac.ADSB.GS))
		}

		if ac.ADSB.AltBaro != 0 {
			builder.WriteString(fmt.Sprintf(", alt: %.0f ft", ac.ADSB.AltBaro))
		}

		if ac.ADSB.BaroRate != 0 {
			builder.WriteString(fmt.Sprintf(", VS: %.0f fpm", ac.ADSB.BaroRate))
		}

		// Add squawk code if available
		if ac.ADSB.Squawk != "" {
			builder.WriteString(fmt.Sprintf(", squawk: %s", ac.ADSB.Squawk))
		}

		builder.WriteString(", status: airborne")

		// Add takeoff time - always show, use N/A if unknown
		if ac.DateTookoff != nil {
			timeSince := time.Since(*ac.DateTookoff)
			builder.WriteString(fmt.Sprintf(", T/O: %s ago", formatDuration(timeSince)))
		} else {
			builder.WriteString(", T/O: N/A")
		}
	}

	// Airport position (distance and bearing to airport)
	if ac.Distance != nil && ac.ADSB != nil && ac.ADSB.Lat != 0 && ac.ADSB.Lon != 0 {
		bearingToStation := adsb.CalculateBearing(ac.ADSB.Lat, ac.ADSB.Lon, airport.Coordinates[0], airport.Coordinates[1])
		bearingFromStation := bearingToStation + 180
		if bearingFromStation >= 360 {
			bearingFromStation -= 360
		}
		builder.WriteString(fmt.Sprintf(" | Airport position: %.1f NM, heading %.0f° | BFS (airport->plane): %.0f°", *ac.Distance, bearingToStation, bearingFromStation))
	}

	// Flight phase
	if ac.Phase != nil && len(ac.Phase.Current) > 0 {
		currentPhase := ac.Phase.Current[0]
		fullPhaseName := getFullPhaseName(currentPhase.Phase)
		timeSince := time.Since(currentPhase.Timestamp)
		builder.WriteString(fmt.Sprintf(" | Phase: %s (%s)", fullPhaseName, formatDuration(timeSince)))
	}

	// Telemetry status
	builder.WriteString(fmt.Sprintf(" | Telemetry: %s", ac.Status))
	if !ac.LastSeen.IsZero() {
		timeSince := time.Since(ac.LastSeen)
		builder.WriteString(fmt.Sprintf(", Last seen: %s", formatDuration(timeSince)))
	}

	return builder.String()
}

// formatGroundAircraft formats a single ground aircraft for display
func formatGroundAircraft(ac *adsb.Aircraft, airport AirportInfo) string {
	var builder strings.Builder

	// Basic info - callsign and operator
	callsign := ac.Flight
	if callsign == "" {
		callsign = "Unknown"
	}

	builder.WriteString(fmt.Sprintf("%s", callsign))

	// Operator from ADSB data
	if ac.Airline != "" {
		builder.WriteString(fmt.Sprintf(" (%s)", ac.Airline))
	}

	builder.WriteString(" | ")

	// Aircraft type
	if ac.ADSB != nil && ac.ADSB.AircraftType != "" {
		builder.WriteString(fmt.Sprintf("Type: %s | ", ac.ADSB.AircraftType))
	}

	// Wake category
	if ac.ADSB != nil && ac.ADSB.Category != "" {
		builder.WriteString(fmt.Sprintf("Wake Category: %s | ", ac.ADSB.Category))
	}

	// Flight parameters
	if ac.ADSB != nil {
		builder.WriteString("Flight params: ")

		// Use magnetic heading if available (0-360° are all valid), fallback to track
		if ac.ADSB.MagHeading >= 0 && ac.ADSB.MagHeading <= 360 {
			builder.WriteString(fmt.Sprintf("HDG: %.0f", ac.ADSB.MagHeading))
		} else if ac.ADSB.Track != 0 {
			builder.WriteString(fmt.Sprintf("HDG: %.0f", ac.ADSB.Track))
		}

		if ac.ADSB.TAS != 0 {
			builder.WriteString(fmt.Sprintf(", TAS: %.0f kts", ac.ADSB.TAS))
		}

		if ac.ADSB.GS != 0 {
			builder.WriteString(fmt.Sprintf(", GS: %.0f kts", ac.ADSB.GS))
		}

		if ac.ADSB.AltBaro != 0 {
			builder.WriteString(fmt.Sprintf(", alt: %.0f ft", ac.ADSB.AltBaro))
		}

		if ac.ADSB.BaroRate != 0 {
			builder.WriteString(fmt.Sprintf(", VS: %.0f fpm", ac.ADSB.BaroRate))
		}

		// Add squawk code if available
		if ac.ADSB.Squawk != "" {
			builder.WriteString(fmt.Sprintf(", squawk: %s", ac.ADSB.Squawk))
		}

		builder.WriteString(", status: on ground")

		// Add takeoff time - always show, use N/A if unknown
		if ac.DateTookoff != nil {
			timeSince := time.Since(*ac.DateTookoff)
			builder.WriteString(fmt.Sprintf(", T/O: %s ago", formatDuration(timeSince)))
		} else {
			builder.WriteString(", T/O: N/A")
		}
	}

	// Flight phase
	if ac.Phase != nil && len(ac.Phase.Current) > 0 {
		currentPhase := ac.Phase.Current[0]
		fullPhaseName := getFullPhaseName(currentPhase.Phase)
		timeSince := time.Since(currentPhase.Timestamp)
		builder.WriteString(fmt.Sprintf(" | Phase: %s (%s)", fullPhaseName, formatDuration(timeSince)))
	}

	// Telemetry status
	builder.WriteString(fmt.Sprintf(" | Telemetry: %s", ac.Status))
	if !ac.LastSeen.IsZero() {
		timeSince := time.Since(ac.LastSeen)
		builder.WriteString(fmt.Sprintf(", Last seen: %s", formatDuration(timeSince)))
	}

	return builder.String()
}

// getFullPhaseName converts phase codes to full names
func getFullPhaseName(phase string) string {
	switch phase {
	case "NEW":
		return "New"
	case "TAX":
		return "Taxiing"
	case "T/O":
		return "Takeoff"
	case "DEP":
		return "Departure"
	case "CRZ":
		return "Cruise"
	case "ARR":
		return "Arrival"
	case "APP":
		return "Approach"
	case "T/D":
		return "Touchdown"
	default:
		return phase // Return original if not recognized
	}
}

// FormatWeatherData formats weather data for template rendering
func FormatWeatherData(weather *weather.WeatherData) string {
	if weather == nil {
		return "Weather data not available."
	}

	var builder strings.Builder

	// Extract latest METAR - only show decoded, not raw
	if weather.METAR != nil {
		if metarMap, ok := weather.METAR.(map[string]interface{}); ok {
			if trend, exists := metarMap["trend"]; exists {
				if trendSlice, ok := trend.([]interface{}); ok && len(trendSlice) > 0 {
					// Get the latest METAR (first in trend array)
					if latestMetar, ok := trendSlice[0].(map[string]interface{}); ok {
						if txt, exists := latestMetar["txt"]; exists {
							if txtSlice, ok := txt.([]interface{}); ok && len(txtSlice) > 0 {
								builder.WriteString(fmt.Sprintf("Current Weather: %v\n", txtSlice[0]))
							}
						}
					}
				}
			}
		}
	}

	// TAF summary (keep this but simplified)
	if weather.TAF != nil {
		if tafMap, ok := weather.TAF.(map[string]interface{}); ok {
			if _, exists := tafMap["decoded"]; exists {
				builder.WriteString("TAF Summary: Terminal forecast available\n")
			}
		}
	}

	// Last updated
	if !weather.LastUpdated.IsZero() {
		timeSince := time.Since(weather.LastUpdated)
		builder.WriteString(fmt.Sprintf("Last updated: %s ago\n", formatDuration(timeSince)))
	}

	return builder.String()
}

// FormatRunwayData formats runway data for template rendering
func FormatRunwayData(runways []RunwayInfo) string {
	if len(runways) == 0 {
		return "Runway information not available."
	}

	var builder strings.Builder

	for _, runway := range runways {
		builder.WriteString(fmt.Sprintf("• Runway %s", runway.Name))
		if runway.LengthFt > 0 {
			builder.WriteString(fmt.Sprintf(" (%d ft)", runway.LengthFt))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

// FormatTranscriptionHistory formats recent communications for template rendering
func FormatTranscriptionHistory(communications []TranscriptionSummary) string {
	if len(communications) == 0 {
		return "No recent radio communications available."
	}

	var builder strings.Builder
	builder.WriteString("RECENT RADIO COMMUNICATIONS (last 10 minutes):\n\n")

	for _, comm := range communications {
		timeSince := time.Since(comm.Timestamp)
		builder.WriteString(fmt.Sprintf("• [%s ago] %s", formatDuration(timeSince), comm.Frequency))
		if comm.Speaker != "" {
			builder.WriteString(fmt.Sprintf(" (%s)", comm.Speaker))
		}
		if comm.Callsign != "" {
			builder.WriteString(fmt.Sprintf(" [%s]", comm.Callsign))
		}
		builder.WriteString(fmt.Sprintf(": %s\n", comm.Content))
	}

	return builder.String()
}

// FormatAirportData formats airport information for template rendering
func FormatAirportData(airport AirportInfo) string {
	var builder strings.Builder
	builder.WriteString("AIRPORT INFORMATION:\n\n")
	builder.WriteString(fmt.Sprintf("• %s (%s)\n", airport.Name, airport.Code))
	if len(airport.Coordinates) >= 2 {
		builder.WriteString(fmt.Sprintf("• Coordinates: %.4f°, %.4f°\n", airport.Coordinates[0], airport.Coordinates[1]))
	}
	if airport.ElevationFt > 0 {
		builder.WriteString(fmt.Sprintf("• Elevation: %d ft MSL\n", airport.ElevationFt))
	}
	return builder.String()
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}
