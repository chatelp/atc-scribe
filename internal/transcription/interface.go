package transcription

// ProcessorInterface defines the interface for audio transcription processors
type ProcessorInterface interface {
	Start() error
	Stop() error
}

// Ensure the processor implements the interface
var _ ProcessorInterface = (*Processor)(nil)
