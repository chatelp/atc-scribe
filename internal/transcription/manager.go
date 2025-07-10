package transcription

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/audio"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// TranscriptionManager manages transcription processors for frequencies
type TranscriptionManager struct {
	processors           map[string]ProcessorInterface
	mu                   sync.RWMutex
	wsServer             *websocket.Server
	transcriptionStorage *sqlite.TranscriptionStorage
	aircraftStorage      *sqlite.AircraftStorage
	clearanceStorage     *sqlite.ClearanceStorage
	logger               *logger.Logger
	openAIAPIKey         string
	transcriptionConfig  Config
	postProcessor        *PostProcessor
	postProcessingConfig PostProcessingConfig
	templateRenderer     TemplateRenderer
	frequencyNames       map[string]string // Map of frequency IDs to names
}

// NewTranscriptionManager creates a new transcription manager
func NewTranscriptionManager(
	wsServer *websocket.Server,
	transcriptionStorage *sqlite.TranscriptionStorage,
	aircraftStorage *sqlite.AircraftStorage,
	clearanceStorage *sqlite.ClearanceStorage,
	logger *logger.Logger,
	openAIAPIKey string,
	transcriptionConfig Config,
	postProcessingConfig PostProcessingConfig,
	templateRenderer TemplateRenderer,
	frequencyConfigs []FrequencyConfig,
) *TranscriptionManager {
	// Create map of frequency IDs to names
	frequencyNames := make(map[string]string)
	for _, freq := range frequencyConfigs {
		frequencyNames[freq.ID] = freq.Name
	}

	return &TranscriptionManager{
		processors:           make(map[string]ProcessorInterface),
		wsServer:             wsServer,
		transcriptionStorage: transcriptionStorage,
		aircraftStorage:      aircraftStorage,
		clearanceStorage:     clearanceStorage,
		logger:               logger,
		openAIAPIKey:         openAIAPIKey,
		transcriptionConfig:  transcriptionConfig,
		postProcessingConfig: postProcessingConfig,
		templateRenderer:     templateRenderer,
		frequencyNames:       frequencyNames,
	}
}

// FrequencyConfig represents a frequency configuration
type FrequencyConfig struct {
	ID   string
	Name string
}

// StartTranscription starts transcription for a frequency
func (m *TranscriptionManager) StartTranscription(
	ctx context.Context,
	frequencyID string,
	frequencyName string,
	audioURL string,
	transcribeAudio bool,
) error {
	// Skip if transcription is not enabled for this frequency
	if !transcribeAudio {
		m.logger.Info("Transcription not enabled for frequency",
			logger.String("id", frequencyID),
			logger.String("name", frequencyName))
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if processor already exists
	if _, exists := m.processors[frequencyID]; exists {
		m.logger.Info("Transcription already started for frequency",
			logger.String("id", frequencyID),
			logger.String("name", frequencyName))
		return nil
	}

	m.logger.Info("Starting transcription for frequency",
		logger.String("id", frequencyID),
		logger.String("name", frequencyName),
		logger.String("url", audioURL))

	// Create a CentralAudioProcessor for this frequency
	audioConfig := audio.CentralProcessorConfig{
		FFmpegPath:               m.transcriptionConfig.FFmpegPath,
		SampleRate:               m.transcriptionConfig.FFmpegSampleRate,
		Channels:                 m.transcriptionConfig.FFmpegChannels,
		Format:                   m.transcriptionConfig.FFmpegFormat,
		ReconnectDelay:           time.Duration(m.transcriptionConfig.ReconnectIntervalSec) * time.Second,
		FFmpegTimeoutSecs:        0, // Default no timeout for transcription
		FFmpegReconnectDelaySecs: 2, // Default reconnect delay for transcription
	}

	audioProcessor, err := audio.NewCentralAudioProcessor(
		ctx,
		frequencyID,
		audioURL,
		audioConfig,
		m.logger.Named("audio"),
	)
	if err != nil {
		return fmt.Errorf("failed to create audio processor: %w", err)
	}

	// Start the audio processor
	if err := audioProcessor.Start(); err != nil {
		return fmt.Errorf("failed to start audio processor: %w", err)
	}

	// Create a reader from the audio processor
	reader, err := audioProcessor.CreateReader(fmt.Sprintf("transcription-%s", frequencyID))
	if err != nil {
		audioProcessor.Stop()
		return fmt.Errorf("failed to create audio reader: %w", err)
	}

	// Create a processor that uses the reader
	processor, err := NewProcessor(
		ctx,
		frequencyID,
		reader,
		m.transcriptionConfig,
		m.wsServer,
		m.transcriptionStorage,
		m.logger,
	)
	if err != nil {
		return err
	}

	// Start processor
	if err := processor.Start(); err != nil {
		return err
	}

	// Store processor
	m.processors[frequencyID] = processor

	return nil
}

// StartTranscriptionWithExternalAudio starts transcription for a frequency using an external audio processor
func (m *TranscriptionManager) StartTranscriptionWithExternalAudio(
	ctx context.Context,
	frequencyID string,
	frequencyName string,
	transcribeAudio bool,
	audioProcessor interface{},
) error {
	// Skip if transcription is not enabled for this frequency
	if !transcribeAudio {
		m.logger.Info("Transcription not enabled for frequency",
			logger.String("id", frequencyID),
			logger.String("name", frequencyName))
		return nil
	}

	// Skip if no OpenAI API key is provided
	if m.openAIAPIKey == "" {
		m.logger.Info("Transcription disabled - no OpenAI API key provided",
			logger.String("id", frequencyID),
			logger.String("name", frequencyName))
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if processor already exists
	if _, exists := m.processors[frequencyID]; exists {
		m.logger.Info("Transcription already started for frequency",
			logger.String("id", frequencyID),
			logger.String("name", frequencyName))
		return nil
	}

	m.logger.Info("Starting transcription with external audio for frequency",
		logger.String("id", frequencyID),
		logger.String("name", frequencyName))

	// Create external processor based on the type of audio processor
	var processor ProcessorInterface
	var err error

	// We only support CentralAudioProcessor now
	ap, ok := audioProcessor.(*audio.CentralAudioProcessor)
	if !ok {
		return fmt.Errorf("unsupported audio processor type: %T, only CentralAudioProcessor is supported", audioProcessor)
	}

	// Create a reader from the central processor
	reader, readerErr := ap.CreateReader(fmt.Sprintf("transcription-%s", frequencyID))
	if readerErr != nil {
		return fmt.Errorf("failed to create reader from central processor: %w", readerErr)
	}

	// Create a processor that uses the reader
	processor, err = NewProcessor(
		ctx,
		frequencyID,
		reader,
		m.transcriptionConfig,
		m.wsServer,
		m.transcriptionStorage,
		m.logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create external processor: %w", err)
	}

	// Start processor
	if err := processor.Start(); err != nil {
		return fmt.Errorf("failed to start external processor: %w", err)
	}

	// Store processor
	m.processors[frequencyID] = processor

	return nil
}

// StopTranscription stops transcription for a frequency
func (m *TranscriptionManager) StopTranscription(frequencyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if processor exists
	processor, exists := m.processors[frequencyID]
	if !exists {
		m.logger.Info("No transcription processor found for frequency", logger.String("id", frequencyID))
		return
	}

	m.logger.Info("Stopping transcription for frequency", logger.String("id", frequencyID))

	// Stop processor
	if err := processor.Stop(); err != nil {
		m.logger.Error("Error stopping transcription processor",
			logger.String("id", frequencyID),
			logger.Error(err))
	}

	// Remove processor
	delete(m.processors, frequencyID)
}

// StopAllTranscriptions stops all transcription processors and post-processing
func (m *TranscriptionManager) StopAllTranscriptions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Stopping all transcription processors", logger.Int("count", len(m.processors)))

	// Stop all processors
	for id, processor := range m.processors {
		if err := processor.Stop(); err != nil {
			m.logger.Error("Error stopping transcription processor",
				logger.String("id", id),
				logger.Error(err))
		}
	}

	// Clear processors
	m.processors = make(map[string]ProcessorInterface)

	// Stop post-processor
	m.StopPostProcessing()
}

// StartPostProcessing starts the post-processing of transcriptions
func (m *TranscriptionManager) StartPostProcessing(ctx context.Context) error {
	if m.postProcessor != nil {
		m.logger.Info("Post-processing already started")
		return nil
	}

	// Skip if no OpenAI API key is provided
	if m.openAIAPIKey == "" {
		m.logger.Info("Post-processing disabled - no OpenAI API key provided")
		return nil
	}

	// Create OpenAI client for post-processing
	openaiClient := NewOpenAIClient(m.openAIAPIKey, m.postProcessingConfig.Model, m.postProcessingConfig.TimeoutSeconds, m.logger)

	// Create post-processor
	var err error
	m.postProcessor, err = NewPostProcessor(
		ctx,
		m.transcriptionStorage,
		m.aircraftStorage,
		m.clearanceStorage,
		openaiClient,
		m.wsServer,
		m.templateRenderer,
		m.postProcessingConfig,
		m.logger,
		m.frequencyNames,
	)
	if err != nil {
		return fmt.Errorf("failed to create post-processor: %w", err)
	}

	// Start post-processor
	if err := m.postProcessor.Start(); err != nil {
		return fmt.Errorf("failed to start post-processor: %w", err)
	}

	m.logger.Info("Post-processing started")
	return nil
}

// StopPostProcessing stops the post-processing of transcriptions
func (m *TranscriptionManager) StopPostProcessing() {
	if m.postProcessor == nil {
		m.logger.Info("No post-processor to stop")
		return
	}

	m.logger.Info("Stopping post-processor")
	m.postProcessor.Stop()
	m.postProcessor = nil
}
