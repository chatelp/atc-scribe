package templating

import (
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/weather"
)

// TemplateContext represents the raw data context for template rendering
type TemplateContext struct {
	Aircraft             []*adsb.Aircraft       `json:"aircraft"`
	Weather              *weather.WeatherData   `json:"weather"`
	Runways              []RunwayInfo           `json:"runways"`
	TranscriptionHistory []TranscriptionSummary `json:"transcription_history"`
	Airport              AirportInfo            `json:"airport"`
	Timestamp            time.Time              `json:"timestamp"`
}

// TemplateData represents the formatted data for template rendering
type TemplateData struct {
	Aircraft             string    `json:"aircraft"`
	Weather              string    `json:"weather"`
	Runways              string    `json:"runways"`
	TranscriptionHistory string    `json:"transcription_history"` // Only populated for ATC Chat
	Airport              string    `json:"airport"`
	Time                 string    `json:"time"`
	Timestamp            time.Time `json:"timestamp"`
}

// FormattingOptions controls what data is included and how it's formatted
type FormattingOptions struct {
	MaxAircraft                 int    `json:"max_aircraft"`
	IncludeWeather              bool   `json:"include_weather"`
	IncludeRunways              bool   `json:"include_runways"`
	IncludeTranscriptionHistory bool   `json:"include_transcription_history"` // Only for ATC Chat
	TimeFormat                  string `json:"time_format"`
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

// TranscriptionSummary represents recent radio communications for templating
type TranscriptionSummary struct {
	Timestamp time.Time `json:"timestamp"`
	Frequency string    `json:"frequency"`
	Content   string    `json:"content"`
	Speaker   string    `json:"speaker"`
	Callsign  string    `json:"callsign,omitempty"`
}

// DefaultFormattingOptions returns sensible defaults for template formatting
func DefaultFormattingOptions() FormattingOptions {
	return FormattingOptions{
		MaxAircraft:                 50,
		IncludeWeather:              true,
		IncludeRunways:              true,
		IncludeTranscriptionHistory: false, // Default to false, enable explicitly for ATC Chat
		TimeFormat:                  "Monday, January 2, 2006 at 15:04:05 UTC",
	}
}

// ATCChatFormattingOptions returns formatting options optimized for ATC Chat
func ATCChatFormattingOptions() FormattingOptions {
	opts := DefaultFormattingOptions()
	opts.IncludeTranscriptionHistory = true
	opts.MaxAircraft = 200 // Use a high default, will be overridden by config
	return opts
}

// PostProcessorFormattingOptions returns formatting options optimized for Post-Processor
func PostProcessorFormattingOptions() FormattingOptions {
	opts := DefaultFormattingOptions()
	opts.IncludeTranscriptionHistory = false // Post-processor gets transcripts in user input
	opts.MaxAircraft = 100
	return opts
}
