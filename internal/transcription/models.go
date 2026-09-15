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

// Backend names for the transcription engine.
const (
	BackendOpenAI = "openai"
	BackendLocal  = "local"
)

// LocalSTTConfig configures the local speech-to-text sidecar backend.
//
// The sidecar receives whole transmissions rather than a continuous stream, so
// this side has to decide where a transmission begins and ends. On an SDR feed
// that is straightforward: RTLSDR-Airband writes digital silence between
// transmissions, so squelch activity is unambiguous.
type LocalSTTConfig struct {
	ServerURL      string `toml:"server_url"`      // Base URL of the sidecar, e.g. http://127.0.0.1:8178
	TimeoutSeconds int    `toml:"timeout_seconds"` // HTTP timeout for one transmission

	SegmentSilenceMs  int     `toml:"segment_silence_ms"`  // Silence that ends a transmission (default 600)
	SegmentMinMs      int     `toml:"segment_min_ms"`      // Shorter than this is discarded (default 400)
	SegmentMaxSeconds int     `toml:"segment_max_seconds"` // Hard bound on one segment (default 30)
	SegmentPrerollMs  int     `toml:"segment_preroll_ms"`  // Audio kept before speech starts (default 200)
	SilenceThreshold  float64 `toml:"silence_threshold"`   // RMS below which a frame is silence (default 0.005)
}

// Config represents the configuration for the transcription service
type Config struct {
	Backend               string // "openai" or "local"
	Local                 LocalSTTConfig
	FrequencyLanguages    map[string]string // frequency id -> expected language, from the catalogue
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
	LogDir                string // Optional directory for transcription log files
}
