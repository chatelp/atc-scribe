package transcription

import (
	"time"
)

// TranscriptionEvent represents a transcription event
type TranscriptionEvent struct {
	Type      string    // "delta" or "completed"
	Text      string    // The transcription text
	Timestamp time.Time // When the event occurred
}

// Config represents the configuration for the transcription service
type Config struct {
	OpenAIAPIKey          string
	Model                 string
	Language              string
	NoiseReduction        string
	ChunkMs               int
	BufferSizeKB          int
	FFmpegPath            string
	FFmpegSampleRate      int
	FFmpegChannels        int
	FFmpegFormat          string
	ReconnectIntervalSec  int
	MaxRetries            int
	TurnDetectionType     string
	PrefixPaddingMs       int
	SilenceDurationMs     int
	VADThreshold          float64
	RetryMaxAttempts      int
	RetryInitialBackoffMs int
	RetryMaxBackoffMs     int
	PromptPath            string
	Prompt                string // Loaded from PromptPath
	TimeoutSeconds        int    // HTTP timeout for OpenAI API requests
}
