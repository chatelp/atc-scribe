package frequencies

import (
	"context"
	"io"
	"time"
)

// Frequency represents a monitored ATC frequency
type Frequency struct {
	ID              string    `json:"id"`
	Airport         string    `json:"airport"`
	Name            string    `json:"name"`
	FrequencyMHz    float64   `json:"frequency_mhz"`
	URL             string    `json:"url"`
	Status          string    `json:"status"` // "active", "connecting", "error"
	LastError       string    `json:"last_error,omitempty"`
	Bitrate         int       `json:"bitrate,omitempty"`
	Format          string    `json:"format,omitempty"`
	StreamURL       string    `json:"stream_url"` // Relative URL to stream from our server
	LastActive      time.Time `json:"last_active,omitempty"`
	Order           int       `json:"order"`            // Order for display/sorting
	TranscribeAudio bool      `json:"transcribe_audio"` // Whether to transcribe audio for this frequency
}

// Stream represents the resources for a single active client's connection to an audio feed.
// It is NOT a shared resource in this model.
type Stream struct {
	Reader         io.ReadCloser // The client's dedicated reader (now from audio.Processor)
	OriginalStream io.ReadCloser // The client's dedicated connection to audio source (kept for backward compatibility)
	ContentType    string
	Bitrate        int                // Metadata for the stream
	Format         string             // Metadata for the stream
	processCancel  context.CancelFunc // Cancels the goroutine copying from OriginalStream to Reader for this client
}

// FrequencyResponse represents the API response for frequency data
type FrequencyResponse struct {
	Timestamp   time.Time   `json:"timestamp"`
	Count       int         `json:"count"`
	Frequencies []Frequency `json:"frequencies"`
}

// StreamOptions contains options for streaming
type StreamOptions struct {
	URL        string
	BufferSize int
	Timeout    time.Duration
}

// StreamMetadata contains metadata about an audio stream
type StreamMetadata struct {
	ContentType string
	Bitrate     int
	Format      string
	Description string
	Genre       string
	Name        string
}
