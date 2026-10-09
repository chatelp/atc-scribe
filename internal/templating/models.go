package templating

import (
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/weather"
)

// TemplateContext represents the raw data context for template rendering
type TemplateContext struct {
	Aircraft      []*adsb.Aircraft     `json:"aircraft"`
	Weather       *weather.WeatherData `json:"weather"`
	Runways       []RunwayInfo         `json:"runways"`
	ActiveRunways []adsb.RunwayScore   `json:"active_runways"`
	Airport       AirportInfo          `json:"airport"`
	Timestamp     time.Time            `json:"timestamp"`
}

// TemplateData represents the formatted data for template rendering
type TemplateData struct {
	Aircraft             string    `json:"aircraft"`
	Weather              string    `json:"weather"`
	Runways              string    `json:"runways"`
	ActiveRunways        string    `json:"active_runways"`
	TranscriptionHistory string    `json:"transcription_history"` // always empty since the voice chat went; kept so a template naming it still renders
	Airport              string    `json:"airport"`
	Time                 string    `json:"time"`
	Timestamp            time.Time `json:"timestamp"`
}

// FormattingOptions controls what data is included and how it's formatted
type FormattingOptions struct {
	MaxAircraft    int    `json:"max_aircraft"`
	IncludeWeather bool   `json:"include_weather"`
	IncludeRunways bool   `json:"include_runways"`
	TimeFormat     string `json:"time_format"`
}

// AirportInfo represents airport information for templating
type AirportInfo struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Coordinates []float64 `json:"coordinates"`
	ElevationFt int       `json:"elevation_ft"`
}

// RunwayInfo represents runway information for templating
type RunwayInfo struct {
	Name       string   `json:"name"`
	Heading    int      `json:"heading"`
	LengthFt   int      `json:"length_ft"`
	Active     bool     `json:"active"`
	Operations []string `json:"operations"`
}

// DefaultFormattingOptions returns sensible defaults for template formatting
func DefaultFormattingOptions() FormattingOptions {
	return FormattingOptions{
		MaxAircraft:    50,
		IncludeWeather: true,
		IncludeRunways: true,
		TimeFormat:     "Monday, January 2, 2006 at 15:04:05 UTC",
	}
}

// PostProcessorFormattingOptions returns formatting options optimized for Post-Processor
func PostProcessorFormattingOptions() FormattingOptions {
	opts := DefaultFormattingOptions()
	opts.MaxAircraft = 100
	return opts
}
