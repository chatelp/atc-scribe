package transcription

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/audio"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// Import the logger package's exported functions
var (
	String = logger.String
	Int    = logger.Int
	Int64  = logger.Int64
	Error  = logger.Error
)

// Processor manages transcription for a specific frequency using a provided reader
type Processor struct {
	frequencyID         string
	audioReader         io.ReadCloser
	openaiClient        *OpenAIClient
	wsServer            *websocket.Server
	storage             *sqlite.TranscriptionStorage
	ctx                 context.Context
	cancel              context.CancelFunc
	logger              *logger.Logger
	audioChunker        *audio.AudioChunker
	sessionID           string
	clientSecret        string
	wsConn              *OpenAIWebSocketConn
	chunkCount          int
	chunkCountMu        sync.Mutex
	transcriptionConfig Config
	sessionStartTime    time.Time
	sessionRefreshMu    sync.Mutex
}

// NewProcessor creates a new transcription processor with a provided reader
func NewProcessor(
	ctx context.Context,
	frequencyID string,
	audioReader io.ReadCloser,
	config Config,
	wsServer *websocket.Server,
	storage *sqlite.TranscriptionStorage,
	logger *logger.Logger,
) (ProcessorInterface, error) {
	// Check if OpenAI API key is provided - fail fast if missing
	if config.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required for transcription processor")
	}

	procCtx, procCancel := context.WithCancel(ctx)

	// Create OpenAI client
	openaiClient := NewOpenAIClient(config.OpenAIAPIKey, config.Model, config.TimeoutSeconds, logger)

	// Create processor
	processor := &Processor{
		frequencyID:         frequencyID,
		audioReader:         audioReader,
		openaiClient:        openaiClient,
		wsServer:            wsServer,
		storage:             storage,
		ctx:                 procCtx,
		cancel:              procCancel,
		logger:              logger.Named("custom-xscribe").With(String("frequency_id", frequencyID)),
		audioChunker:        audio.NewAudioChunker(config.FFmpegSampleRate, config.FFmpegChannels, config.ChunkMs),
		transcriptionConfig: config,
	}

	return processor, nil
}

// Start starts the transcription processor
func (p *Processor) Start() error {
	p.logger.Info("Starting custom transcription processor",
		String("frequency_id", p.frequencyID))

	// Create OpenAI transcription session
	var err error
	p.sessionID, p.clientSecret, err = p.openaiClient.CreateSession(p.ctx, p.transcriptionConfig)
	if err != nil {
		p.audioReader.Close()
		return fmt.Errorf("failed to create transcription session: %w", err)
	}
	p.logger.Info("Created transcription session", String("session_id", p.sessionID))

	// Record session start time
	p.sessionStartTime = time.Now()

	// Connect to OpenAI WebSocket
	p.wsConn, err = p.openaiClient.ConnectWebSocket(p.ctx, p.sessionID, p.clientSecret)
	if err != nil {
		p.audioReader.Close()
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	p.logger.Info("Connected to OpenAI WebSocket")

	// Start processing in goroutines
	go p.processAudio()
	go p.processTranscriptions()
	go p.monitorSessionDuration()

	return nil
}

// Stop stops the transcription processor
func (p *Processor) Stop() error {
	p.logger.Info("Stopping custom transcription processor")

	// Cancel context to stop all operations
	p.cancel()

	// Close WebSocket connection
	if p.wsConn != nil {
		p.wsConn.Close()
	}

	// Close audio reader
	if p.audioReader != nil {
		p.audioReader.Close()
	}

	return nil
}

// processAudio processes audio from the reader
func (p *Processor) processAudio() {
	p.logger.Info("Starting audio processing")

	// Create buffer for audio chunks
	buffer := make([]byte, 4096)

	// Track consecutive errors for backoff
	consecutiveErrors := 0
	maxConsecutiveErrors := 5
	reconnectAttempted := false

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Audio processing stopped due to context cancellation")
			return
		default:
			// Read from audio source
			n, err := p.audioReader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					p.logger.Info("Audio source ended")
					return
				}
				p.logger.Error("Error reading from audio source", Error(err))
				return
			}

			if n > 0 {
				// Process audio chunk
				chunks, err := p.audioChunker.ProcessChunk(buffer[:n])
				if err != nil {
					p.logger.Error("Error processing audio chunk", Error(err))
					continue
				}

				// Send chunks to OpenAI
				for _, chunk := range chunks {
					// Base64 encode the chunk
					encoded := base64.StdEncoding.EncodeToString(chunk)

					// Send to OpenAI
					if err := p.sendAudioChunk(encoded); err != nil {
						consecutiveErrors++

						// Log with appropriate level based on consecutive errors
						if consecutiveErrors <= 2 {
							p.logger.Error("Error sending audio chunk",
								Error(err),
								Int("consecutive_errors", consecutiveErrors))
						} else if consecutiveErrors == 3 {
							p.logger.Warn("Multiple consecutive errors sending audio chunks, will attempt reconnection soon",
								Error(err),
								Int("consecutive_errors", consecutiveErrors))
						} else {
							// Only log every 10th error after the initial ones to avoid log spam
							if consecutiveErrors%10 == 0 {
								p.logger.Warn("Continuing to experience audio chunk sending errors",
									Error(err),
									Int("consecutive_errors", consecutiveErrors))
							}
						}

						// After several consecutive errors, try to reconnect
						if consecutiveErrors >= maxConsecutiveErrors && !reconnectAttempted {
							p.logger.Info("Too many consecutive errors, attempting to reconnect WebSocket")

							// Try to reconnect
							if err := p.reconnectOpenAI(); err != nil {
								p.logger.Error("Failed to reconnect to OpenAI", Error(err))
								reconnectAttempted = true // Only try once per error burst
							} else {
								p.logger.Info("Successfully reconnected to OpenAI")
								consecutiveErrors = 0
								reconnectAttempted = false
							}
						}

						// Add a small delay to avoid hammering the service
						if consecutiveErrors > 0 {
							// Exponential backoff with a cap
							backoffMs := 100 * (1 << uint(min(consecutiveErrors-1, 6))) // Cap at 6.4 seconds
							time.Sleep(time.Duration(backoffMs) * time.Millisecond)
						}

						continue
					}

					// Reset error counter on successful send
					if consecutiveErrors > 0 {
						p.logger.Info("Audio chunk sending recovered after errors",
							Int("previous_consecutive_errors", consecutiveErrors))
						consecutiveErrors = 0
						reconnectAttempted = false
					}
				}
			}
		}
	}
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// sendAudioChunk sends an audio chunk to OpenAI
func (p *Processor) sendAudioChunk(encodedChunk string) error {
	// Create message
	message := map[string]interface{}{
		"type":  "input_audio_buffer.append",
		"audio": encodedChunk,
	}

	// Marshal to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal audio chunk message: %w", err)
	}

	// Log every 100th chunk to avoid excessive logging
	p.chunkCountMu.Lock()
	p.chunkCount++
	chunkCount := p.chunkCount
	p.chunkCountMu.Unlock()

	if chunkCount%100 == 0 {
		p.logger.Debug("Sending audio chunk", Int("chunk_number", chunkCount))
	}

	// Send to OpenAI
	if err := p.wsConn.Send(string(data)); err != nil {
		return fmt.Errorf("failed to send audio chunk: %w", err)
	}

	return nil
}

// processTranscriptions processes transcription events from OpenAI
func (p *Processor) processTranscriptions() {
	p.logger.Info("Starting transcription processing",
		String("frequency_id", p.frequencyID),
		String("session_id", p.sessionID))

	// Track reconnection attempts
	reconnectAttempts := 0
	maxReconnectAttempts := 5
	lastReconnectTime := time.Now()
	reconnectBackoffSeconds := 1

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Transcription processing stopped due to context cancellation")
			return
		default:
			// Receive message from OpenAI
			message, err := p.wsConn.Receive()
			if err != nil {
				// Check if context is canceled or connection is closed
				select {
				case <-p.ctx.Done():
					// This is an expected error during shutdown
					p.logger.Info("WebSocket connection closed during shutdown",
						String("frequency_id", p.frequencyID),
						String("session_id", p.sessionID))
					return
				default:
					// Categorize the error
					isReconnectableError := false
					errorMsg := err.Error()

					// Common WebSocket errors that indicate connection issues
					reconnectableErrors := []string{
						"websocket: close 1000 (normal)",
						"websocket: close 1001 (going away)",
						"websocket: close 1006 (abnormal closure)",
						"websocket: close 1006 (abnormal closure): unexpected EOF",
						"use of closed network connection",
						"connection reset by peer",
						"EOF",
						"websocket: close sent",
						"websocket: close received",
						"i/o timeout",
						"read: connection reset by peer",
					}

					for _, reconnectErr := range reconnectableErrors {
						if errorMsg == reconnectErr || strings.Contains(errorMsg, reconnectErr) {
							isReconnectableError = true
							break
						}
					}

					// Log the error with appropriate level
					if isReconnectableError {
						p.logger.Warn("WebSocket connection issue detected",
							Error(err),
							String("frequency_id", p.frequencyID),
							String("session_id", p.sessionID),
							Int("reconnect_attempts", reconnectAttempts))
					} else {
						p.logger.Error("Error receiving WebSocket message",
							Error(err),
							String("frequency_id", p.frequencyID),
							String("session_id", p.sessionID))
					}

					// Don't immediately return on network errors during shutdown
					if p.ctx.Err() != nil {
						return
					}

					// For reconnectable errors, try to reconnect with backoff
					if isReconnectableError {
						// Check if we've exceeded max reconnect attempts
						if reconnectAttempts >= maxReconnectAttempts {
							timeSinceLastReconnect := time.Since(lastReconnectTime)
							// Reset counter if it's been a while since last reconnect attempt
							if timeSinceLastReconnect > time.Minute*5 {
								p.logger.Info("Resetting reconnection counter after cooling period",
									String("frequency_id", p.frequencyID))
								reconnectAttempts = 0
								reconnectBackoffSeconds = 1
							} else {
								p.logger.Error("Exceeded maximum reconnection attempts",
									String("frequency_id", p.frequencyID),
									Int("max_attempts", maxReconnectAttempts))
								return
							}
						}

						// Apply exponential backoff
						backoffDuration := time.Duration(reconnectBackoffSeconds) * time.Second
						p.logger.Info("WebSocket connection closed, waiting before reconnect attempt",
							String("frequency_id", p.frequencyID),
							String("backoff_duration", backoffDuration.String()),
							Int("attempt", reconnectAttempts+1))

						time.Sleep(backoffDuration)

						// Attempt to reconnect
						if err := p.reconnectOpenAI(); err != nil {
							reconnectAttempts++
							reconnectBackoffSeconds = min(reconnectBackoffSeconds*2, 60) // Cap at 60 seconds
							p.logger.Error("Failed to reconnect to OpenAI",
								Error(err),
								Int("reconnect_attempts", reconnectAttempts),
								Int("next_backoff_seconds", reconnectBackoffSeconds))
						} else {
							p.logger.Info("Successfully reconnected to OpenAI WebSocket",
								String("frequency_id", p.frequencyID),
								String("session_id", p.sessionID))
							reconnectAttempts = 0
							reconnectBackoffSeconds = 1
							lastReconnectTime = time.Now()
						}
						continue
					}

					// For other unexpected errors, return
					return
				}
			}

			// Reset reconnect attempts on successful message
			if reconnectAttempts > 0 {
				reconnectAttempts = 0
				reconnectBackoffSeconds = 1
			}

			// p.logger.Debug("Received message from OpenAI",
			// 	String("frequency_id", p.frequencyID),
			// 	String("message_length", fmt.Sprintf("%d bytes", len(message))))

			// Parse message
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(message), &event); err != nil {
				p.logger.Error("Error parsing event", Error(err))
				continue
			}

			// Get event type
			eventType, ok := event["type"].(string)
			if !ok {
				p.logger.Error("Event missing type field", String("event", message))
				continue
			}

			// Process event based on type
			switch eventType {
			case "conversation.item.input_audio_transcription.delta":
				// Handle partial transcript
				deltaText, ok := event["delta"].(string)
				if !ok {
					p.logger.Error("Delta event missing delta field", String("event", message))
					continue
				}

				// Log the delta but don't send to WebSocket clients
				p.logger.Debug("Received delta transcription",
					String("frequency_id", p.frequencyID),
					String("text", deltaText))

			case "conversation.item.input_audio_transcription.completed":
				// Handle completed transcript
				transcript, ok := event["transcript"].(string)
				if !ok {
					p.logger.Error("Completed event missing transcript field", String("event", message))
					continue
				}

				// Create transcription event
				transcriptionEvent := &TranscriptionEvent{
					Type:      "completed",
					Text:      transcript,
					Timestamp: time.Now().UTC(),
				}

				// Process the event
				if err := p.processTranscriptionEvent(transcriptionEvent); err != nil {
					p.logger.Error("Error processing completed transcription", Error(err))
				}

			case "error":
				// Handle error
				errorObj, ok := event["error"].(map[string]interface{})
				if !ok {
					p.logger.Error("Error event missing error field", String("event", message))
					continue
				}

				errorMessage, ok := errorObj["message"].(string)
				if !ok {
					p.logger.Error("Error object missing message field", String("event", message))
					continue
				}

				p.logger.Error("Received error from OpenAI", String("error", errorMessage))

				// Check if session expired
				errorCode, ok := errorObj["code"].(string)
				if ok && errorCode == "session_expired" {
					p.logger.Info("Session expired, reconnecting")
					if err := p.reconnectOpenAI(); err != nil {
						p.logger.Error("Failed to reconnect to OpenAI", Error(err))
						return
					}
				}
			}
		}
	}
}

// processTranscriptionEvent processes a transcription event
func (p *Processor) processTranscriptionEvent(event *TranscriptionEvent) error {
	// Log the event
	if event.Type == "delta" {
		p.logger.Debug("Received delta transcription", String("text", event.Text))
	} else {
		p.logger.Debug("Received completed transcription", String("text", event.Text))
	}

	// Store completed transcriptions in the database
	if event.Type == "completed" {
		// Create record
		record := &sqlite.TranscriptionRecord{
			FrequencyID:      p.frequencyID,
			CreatedAt:        event.Timestamp,
			Content:          event.Text,
			IsComplete:       true,
			IsProcessed:      false,
			ContentProcessed: "",
			// SpeakerType and Callsign will be empty for now
		}

		// Store in database
		id, err := p.storage.StoreTranscription(record)
		if err != nil {
			return fmt.Errorf("failed to store transcription: %w", err)
		}

		p.logger.Debug("Stored transcription in database", Int64("id", id))

		// Update the record with the ID
		record.ID = id

		// Send to WebSocket clients
		message := &websocket.Message{
			Type: "transcription",
			Data: map[string]interface{}{
				"id":                id,
				"frequency_id":      p.frequencyID,
				"text":              event.Text,
				"timestamp":         event.Timestamp,
				"is_complete":       event.Type == "completed",
				"is_processed":      false,
				"content_processed": "",
			},
		}

		p.logger.Debug("Broadcasting transcription to WebSocket clients",
			String("frequency_id", p.frequencyID),
			String("text", event.Text),
			String("type", event.Type),
			Int64("id", id),
			String("timestamp", event.Timestamp.Format(time.RFC3339)))

		p.wsServer.Broadcast(message)

		return nil
	}

	// For delta transcriptions, just send to WebSocket clients without storing in DB
	message := &websocket.Message{
		Type: "transcription",
		Data: map[string]interface{}{
			"frequency_id":      p.frequencyID,
			"text":              event.Text,
			"timestamp":         event.Timestamp,
			"is_complete":       event.Type == "completed",
			"is_processed":      false,
			"content_processed": "",
		},
	}

	p.wsServer.Broadcast(message)

	return nil
}

// reconnectOpenAI reconnects to OpenAI
func (p *Processor) reconnectOpenAI() error {
	p.sessionRefreshMu.Lock()
	defer p.sessionRefreshMu.Unlock()

	// Close existing connection
	if p.wsConn != nil {
		p.wsConn.Close()
	}

	// Create new session
	var err error
	p.sessionID, p.clientSecret, err = p.openaiClient.CreateSession(p.ctx, p.transcriptionConfig)
	if err != nil {
		return fmt.Errorf("failed to create new transcription session: %w", err)
	}
	p.logger.Info("Created new transcription session", String("session_id", p.sessionID))

	// Reset session start time
	p.sessionStartTime = time.Now()

	// Connect to WebSocket
	p.wsConn, err = p.openaiClient.ConnectWebSocket(p.ctx, p.sessionID, p.clientSecret)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	p.logger.Info("Reconnected to OpenAI WebSocket")

	return nil
}

// monitorSessionDuration monitors the session duration and refreshes it before it expires
func (p *Processor) monitorSessionDuration() {
	// OpenAI sessions expire after 30 minutes, so refresh at 25 minutes to be safe
	sessionRefreshInterval := 25 * time.Minute

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Session monitoring stopped due to context cancellation")
			return
		case <-time.After(1 * time.Minute): // Check every minute
			sessionDuration := time.Since(p.sessionStartTime)

			// If session is approaching expiration, refresh it
			if sessionDuration >= sessionRefreshInterval {
				p.logger.Info("Session approaching expiration, proactively refreshing",
					String("frequency_id", p.frequencyID),
					String("session_duration", sessionDuration.String()),
					String("refresh_interval", sessionRefreshInterval.String()))

				if err := p.reconnectOpenAI(); err != nil {
					p.logger.Error("Failed to proactively refresh session",
						String("frequency_id", p.frequencyID),
						Error(err))
					// Continue monitoring even if refresh fails
				} else {
					p.logger.Info("Successfully refreshed session before expiration",
						String("frequency_id", p.frequencyID))
				}
			}
		}
	}
}
