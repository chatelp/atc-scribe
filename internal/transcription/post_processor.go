package transcription

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// PostProcessingConfig represents configuration for post-processing
type PostProcessingConfig struct {
	Enabled               bool
	Model                 string
	IntervalSeconds       int
	BatchSize             int
	ContextTranscriptions int
	SystemPromptPath      string
	TimeoutSeconds        int
}

// PostProcessingResult represents the structured result from the LLM
type PostProcessingResult struct {
	ProcessedContent string                      `json:"processed_content"`
	SpeakerType      string                      `json:"speaker_type,omitempty"`
	Callsign         string                      `json:"callsign,omitempty"`
	Clearances       []sqlite.ExtractedClearance `json:"clearances,omitempty"`
}

// TemplateRenderer is an interface for rendering templates with airspace data
type TemplateRenderer interface {
	RenderPostProcessorTemplate(templatePath string) (string, error)
}

// PostProcessor manages the post-processing of transcriptions
type PostProcessor struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	transcriptionStorage *sqlite.TranscriptionStorage
	aircraftStorage      *sqlite.AircraftStorage
	clearanceStorage     *sqlite.ClearanceStorage
	openaiClient         *OpenAIClient
	wsServer             *websocket.Server
	templateRenderer     TemplateRenderer
	logger               *logger.Logger
	config               PostProcessingConfig
	processingInterval   time.Duration
	batchSize            int
	wg                   sync.WaitGroup
	frequencyNames       map[string]string // Map of frequency IDs to names
}

// NewPostProcessor creates a new post-processor
func NewPostProcessor(
	ctx context.Context,
	transcriptionStorage *sqlite.TranscriptionStorage,
	aircraftStorage *sqlite.AircraftStorage,
	clearanceStorage *sqlite.ClearanceStorage,
	openaiClient *OpenAIClient,
	wsServer *websocket.Server,
	templateRenderer TemplateRenderer,
	config PostProcessingConfig,
	logger *logger.Logger,
	frequencyNames map[string]string,
) (*PostProcessor, error) {
	// Create context with cancellation
	procCtx, procCancel := context.WithCancel(ctx)

	// Create post-processor
	processor := &PostProcessor{
		ctx:                  procCtx,
		cancel:               procCancel,
		transcriptionStorage: transcriptionStorage,
		aircraftStorage:      aircraftStorage,
		clearanceStorage:     clearanceStorage,
		openaiClient:         openaiClient,
		wsServer:             wsServer,
		templateRenderer:     templateRenderer,
		logger:               logger.Named("post-processor"),
		config:               config,
		processingInterval:   time.Duration(config.IntervalSeconds) * time.Second,
		batchSize:            config.BatchSize,
		frequencyNames:       frequencyNames,
	}

	return processor, nil
}

// Start starts the post-processing loop
func (p *PostProcessor) Start() error {
	if !p.config.Enabled {
		p.logger.Info("Post-processing is disabled, not starting")
		return nil
	}

	p.logger.Info("Starting post-processing loop",
		logger.Int("interval_seconds", p.config.IntervalSeconds),
		logger.Int("batch_size", p.batchSize))

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.processingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				p.logger.Info("Post-processing loop stopped due to context cancellation")
				return
			case <-ticker.C:
				if err := p.processNextBatch(); err != nil {
					p.logger.Error("Error processing batch", logger.Error(err))
				}
			}
		}
	}()
	return nil
}

// Stop stops the post-processing loop
func (p *PostProcessor) Stop() error {
	p.logger.Info("Stopping post-processing loop")
	p.cancel()
	p.wg.Wait()
	return nil
}

// TranscriptionBatch represents a batch of transcriptions to be processed
type TranscriptionBatch struct {
	ID               int64                       `json:"id"`
	Content          string                      `json:"content"`
	ContentProcessed string                      `json:"content_processed"`
	SpeakerType      string                      `json:"speaker_type"`
	Callsign         string                      `json:"callsign"`
	Clearances       []sqlite.ExtractedClearance `json:"clearances"`
	Timestamp        time.Time                   `json:"timestamp"`
}

// processNextBatch processes the next batch of unprocessed transcriptions
func (p *PostProcessor) processNextBatch() error {
	// Get unprocessed transcriptions
	records, err := p.transcriptionStorage.GetUnprocessedTranscriptions(p.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed transcriptions: %w", err)
	}

	if len(records) == 0 {
		p.logger.Debug("No unprocessed transcriptions found")
		return nil // Nothing to process
	}

	p.logger.Debug("Processing batch of transcriptions", logger.Int("count", len(records)))

	// Get frequency name for the first record (assuming all records are from the same frequency)
	var frequencyName string
	var frequencyID string
	if len(records) > 0 {
		frequencyID = records[0].FrequencyID
		var err error
		frequencyName, err = p.getFrequencyName(frequencyID)
		if err != nil {
			p.logger.Error("Failed to get frequency name", logger.Error(err))
			frequencyName = frequencyID // Use ID as fallback
		}
	}

	// Get the last N processed transcriptions for context
	var contextRecords []*sqlite.TranscriptionRecord
	if frequencyID != "" && p.config.ContextTranscriptions > 0 {
		contextRecords, err = p.transcriptionStorage.GetLastProcessedTranscriptions(frequencyID, p.config.ContextTranscriptions)
		if err != nil {
			p.logger.Error("Failed to get context transcriptions", logger.Error(err))
			// Continue without context
		} else {
			p.logger.Debug("Including context transcriptions", logger.Int("count", len(contextRecords)))
		}
	}

	// Prepare batch of transcriptions for processing
	var batch []TranscriptionBatch

	// Add both context and unprocessed transcriptions to the batch
	for _, record := range contextRecords {
		batch = append(batch, TranscriptionBatch{
			ID:               record.ID,
			Content:          record.Content,
			ContentProcessed: record.ContentProcessed,
			SpeakerType:      record.SpeakerType,
			Callsign:         record.Callsign,
			Clearances:       []sqlite.ExtractedClearance{}, // Empty for context records
			Timestamp:        record.CreatedAt,
		})
	}

	for _, record := range records {
		batch = append(batch, TranscriptionBatch{
			ID:               record.ID,
			Content:          record.Content,
			ContentProcessed: "",
			SpeakerType:      "",
			Callsign:         "",
			Clearances:       []sqlite.ExtractedClearance{}, // Will be filled by AI
			Timestamp:        record.CreatedAt,
		})
	}

	// Sort the batch by timestamp (oldest to newest)
	p.sortBatchByTimestamp(batch)

	// Convert batch to JSON
	batchJSON, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal transcription batch: %w", err)
	}

	// Use template renderer to generate system prompt with current airspace data
	systemPrompt, err := p.templateRenderer.RenderPostProcessorTemplate(p.config.SystemPromptPath)
	if err != nil {
		p.logger.Error("Failed to render system prompt template", logger.Error(err))
		// Mark all records as failed to prevent infinite retry
		for _, record := range records {
			if updateErr := p.transcriptionStorage.UpdateProcessedTranscription(
				record.ID,
				"[TEMPLATE_RENDER_FAILED]",
				"UNKNOWN",
				"",
			); updateErr != nil {
				p.logger.Error("Failed to mark transcription as failed",
					logger.Int64("id", record.ID),
					logger.Error(updateErr))
			}
		}
		return err
	}

	// User input contains only the frequency and transcriptions data
	userInput := fmt.Sprintf("Radio Frequency:\n%s\n\nTransmissions Log:\n%s",
		frequencyName,
		string(batchJSON))

	// Process the batch
	results, err := p.processBatch(systemPrompt, userInput)
	if err != nil {
		p.logger.Error("Failed to process batch", logger.Error(err))
		// Mark all records as failed to prevent infinite retry
		for _, record := range records {
			if updateErr := p.transcriptionStorage.UpdateProcessedTranscription(
				record.ID,
				"[PROCESSING_FAILED]",
				"UNKNOWN",
				"",
			); updateErr != nil {
				p.logger.Error("Failed to mark transcription as failed",
					logger.Int64("id", record.ID),
					logger.Error(updateErr))
			}
		}
		return err
	}

	// Check if we got any results
	if len(results) == 0 {
		p.logger.Warn("No results returned from OpenAI API, marking batch as failed")
		// Mark all records as failed to prevent infinite retry
		for _, record := range records {
			if updateErr := p.transcriptionStorage.UpdateProcessedTranscription(
				record.ID,
				"[NO_RESULTS_FROM_API]",
				"UNKNOWN",
				"",
			); updateErr != nil {
				p.logger.Error("Failed to mark transcription as failed",
					logger.Int64("id", record.ID),
					logger.Error(updateErr))
			}
		}
		return nil
	}

	// Update database with processed transcriptions
	for _, result := range results {
		// Skip results with empty processed content or already processed transcriptions (context)
		if result.ContentProcessed == "" {
			p.logger.Warn("Skipping result with empty processed content - this indicates OpenAI returned a result but didn't fill in the content_processed field",
				logger.Int64("id", result.ID),
				logger.String("original_content", result.Content),
				logger.String("speaker_type", result.SpeakerType),
				logger.String("callsign", result.Callsign))
			continue
		}

		// Skip context transcriptions that were already processed
		isContextRecord := false
		for _, contextRecord := range contextRecords {
			if contextRecord.ID == result.ID {
				isContextRecord = true
				break
			}
		}
		if isContextRecord {
			p.logger.Debug("Skipping context record that was already processed",
				logger.Int64("id", result.ID))
			continue
		}

		// Update database
		if err := p.transcriptionStorage.UpdateProcessedTranscription(
			result.ID,
			result.ContentProcessed,
			result.SpeakerType,
			result.Callsign,
		); err != nil {
			p.logger.Error("Failed to update processed transcription",
				logger.Int64("id", result.ID),
				logger.Error(err))
			continue
		}

		// Process clearances if this is an ATC transmission with clearances
		if result.SpeakerType == "ATC" && len(result.Clearances) > 0 {
			for _, clearance := range result.Clearances {
				clearanceRecord := &sqlite.ClearanceRecord{
					TranscriptionID: result.ID,
					Callsign:        clearance.Callsign,
					ClearanceType:   clearance.Type,
					ClearanceText:   clearance.Text,
					Runway:          clearance.Runway,
					Timestamp:       result.Timestamp,
					Status:          "issued",
					CreatedAt:       time.Now().UTC(),
				}

				clearanceID, err := p.clearanceStorage.StoreClearance(clearanceRecord)
				if err != nil {
					p.logger.Error("Failed to store clearance",
						logger.String("callsign", clearance.Callsign),
						logger.String("type", clearance.Type),
						logger.Error(err))
					continue
				}

				// Set the ID for broadcasting
				clearanceRecord.ID = clearanceID

				// Broadcast clearance event via WebSocket
				p.broadcastClearanceEvent(clearanceRecord)

				p.logger.Info("Stored clearance",
					logger.String("callsign", clearance.Callsign),
					logger.String("type", clearance.Type),
					logger.String("runway", clearance.Runway),
					logger.Int64("clearance_id", clearanceID))
			}
		}

		// Find the original record to broadcast
		var record *sqlite.TranscriptionRecord
		for _, r := range records {
			if r.ID == result.ID {
				record = r
				break
			}
		}

		if record == nil {
			p.logger.Error("Failed to find original record for broadcasting",
				logger.Int64("id", result.ID))
			continue
		}

		// Update the record with processed content
		record.ContentProcessed = result.ContentProcessed
		record.SpeakerType = result.SpeakerType
		record.Callsign = result.Callsign
		record.IsProcessed = true

		// Log the processed transcription instead of broadcasting
		p.logProcessedTranscription(record)
	}

	return nil
}

// processBatch processes a batch of transcriptions
func (p *PostProcessor) processBatch(systemPrompt string, userInput string) ([]TranscriptionBatch, error) {
	// Call OpenAI API to process the batch
	results, err := p.openaiClient.PostProcessBatch(p.ctx, systemPrompt, userInput, p.config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to post-process batch: %w", err)
	}

	return results, nil
}

// getFrequencyName retrieves the name of a frequency from its ID
func (p *PostProcessor) getFrequencyName(frequencyID string) (string, error) {
	// Check if we have the frequency name in our cache
	if name, ok := p.frequencyNames[frequencyID]; ok {
		return name, nil
	}

	// If not in cache, try to get it from the database
	// This would require adding a method to get frequency info from the database
	// For now, we'll just return the ID as the name
	return frequencyID, nil
}

// logProcessedTranscription logs a processed transcription to the server console and broadcasts it to WebSocket clients
func (p *PostProcessor) logProcessedTranscription(record *sqlite.TranscriptionRecord) {
	// Log the processed transcription at debug level
	p.logger.Debug("Processed transcription",
		logger.Int64("id", record.ID),
		logger.String("frequency_id", record.FrequencyID),
		logger.String("original_content", record.Content),
		logger.String("processed_content", record.ContentProcessed),
		logger.String("speaker_type", record.SpeakerType),
		logger.String("callsign", record.Callsign),
		logger.Time("timestamp", record.CreatedAt))

	// Create WebSocket message to update the original message
	message := &websocket.Message{
		Type: "transcription_update",
		Data: map[string]interface{}{
			"id":                record.ID,
			"frequency_id":      record.FrequencyID,
			"text":              record.Content,
			"timestamp":         record.CreatedAt,
			"is_complete":       true,
			"is_processed":      true,
			"content_processed": record.ContentProcessed,
			"speaker_type":      record.SpeakerType,
			"callsign":          record.Callsign,
		},
	}

	// Log the message we're about to send
	p.logger.Debug("Broadcasting processed transcription to WebSocket clients",
		logger.Int64("id", record.ID),
		logger.String("frequency_id", record.FrequencyID))

	// Broadcast to WebSocket clients
	p.wsServer.Broadcast(message)
}

// sortBatchByTimestamp sorts a batch of transcriptions by timestamp (oldest to newest)
func (p *PostProcessor) sortBatchByTimestamp(batch []TranscriptionBatch) {
	// Sort the batch by timestamp (ascending order - oldest first)
	sort.Slice(batch, func(i, j int) bool {
		return batch[i].Timestamp.Before(batch[j].Timestamp)
	})
}

// broadcastClearanceEvent broadcasts a clearance event via WebSocket
func (p *PostProcessor) broadcastClearanceEvent(clearance *sqlite.ClearanceRecord) {
	message := &websocket.Message{
		Type: "clearance_issued",
		Data: map[string]interface{}{
			"id":             clearance.ID,
			"callsign":       clearance.Callsign,
			"clearance_type": clearance.ClearanceType,
			"clearance_text": clearance.ClearanceText,
			"runway":         clearance.Runway,
			"timestamp":      clearance.Timestamp,
			"status":         clearance.Status,
		},
	}

	// Log the message we're about to send
	p.logger.Debug("Broadcasting clearance event to WebSocket clients",
		logger.Int64("id", clearance.ID),
		logger.String("callsign", clearance.Callsign),
		logger.String("type", clearance.ClearanceType))

	// Broadcast to WebSocket clients
	p.wsServer.Broadcast(message)
}
