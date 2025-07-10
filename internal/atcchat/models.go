package atcchat

import (
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/weather"
)

// ChatSession represents an active ATC chat session
type ChatSession struct {
	ID              string    `json:"id"`
	OpenAISessionID string    `json:"openai_session_id"`
	ClientSecret    string    `json:"client_secret"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	Active          bool      `json:"active"`
	LastActivity    time.Time `json:"last_activity"`
}

// ChatMessage represents a message in the chat session
type ChatMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	AudioData []byte    `json:"audio_data,omitempty"`
}

// AirspaceContext represents the current airspace data for AI context
type AirspaceContext struct {
	Timestamp            time.Time              `json:"timestamp"`
	Airport              AirportInfo            `json:"airport"`
	Aircraft             []*adsb.Aircraft       `json:"aircraft"`
	Weather              *weather.WeatherData   `json:"weather"`
	Runways              []RunwayInfo           `json:"runways"`
	RecentCommunications []TranscriptionSummary `json:"recent_communications"`
	ActiveSessions       int                    `json:"active_sessions"`
}

// AirportInfo represents airport information
type AirportInfo struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Coordinates []float64 `json:"coordinates"`
	ElevationFt int       `json:"elevation_ft"`
}

// RunwayInfo represents runway information
type RunwayInfo struct {
	Name       string   `json:"name"`
	Heading    int      `json:"heading"`
	LengthFt   int      `json:"length_ft"`
	Active     bool     `json:"active"`
	Operations []string `json:"operations"`
}

// TranscriptionSummary represents recent radio communications
type TranscriptionSummary struct {
	Timestamp time.Time `json:"timestamp"`
	Frequency string    `json:"frequency"`
	Content   string    `json:"content"`
	Speaker   string    `json:"speaker"`
	Callsign  string    `json:"callsign,omitempty"`
}

// PromptData represents data for template rendering
type PromptData struct {
	Aircraft             string `json:"aircraft"`
	Weather              string `json:"weather"`
	Runways              string `json:"runways"`
	TranscriptionHistory string `json:"transcription_history"`
	Timestamp            string `json:"timestamp"`
	Airport              string `json:"airport"`
	Time                 string `json:"time"`
}

// SessionConfig represents configuration for a chat session
type SessionConfig struct {
	InputAudioFormat  string  `json:"input_audio_format"`
	OutputAudioFormat string  `json:"output_audio_format"`
	SampleRate        int     `json:"sample_rate"`
	Channels          int     `json:"channels"`
	MaxResponseTokens int     `json:"max_response_tokens"`
	Temperature       float64 `json:"temperature"`
	Speed             float64 `json:"speed"`
	TurnDetectionType string  `json:"turn_detection_type"`
	VADThreshold      float64 `json:"vad_threshold"`
	SilenceDurationMs int     `json:"silence_duration_ms"`
	Voice             string  `json:"voice"`
	Model             string  `json:"model"`
}

// WebSocketMessage represents messages sent over WebSocket
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// AudioChunk represents an audio data chunk
type AudioChunk struct {
	SessionID string    `json:"session_id"`
	Data      []byte    `json:"data"`
	Format    string    `json:"format"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionStatus represents the status of a chat session
type SessionStatus struct {
	ID           string    `json:"id"`
	Active       bool      `json:"active"`
	Connected    bool      `json:"connected"`
	LastActivity time.Time `json:"last_activity"`
	ExpiresAt    time.Time `json:"expires_at"`
	Error        string    `json:"error,omitempty"`
}
