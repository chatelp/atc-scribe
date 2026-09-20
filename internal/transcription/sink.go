package transcription

import (
	"fmt"
	"time"

	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// transcriptionSink is everything a transcription backend needs downstream of the
// model: persist the text, tell the browser about it, and append to the file log.
//
// Both backends produce the same record, so nothing after this point — the API,
// the post-processor, the map — needs to know where the text came from.
type transcriptionSink struct {
	frequencyID string
	storage     *sqlite.TranscriptionStorage
	wsServer    *websocket.Server
	fileLogger  *FileLogger
	logger      *logger.Logger
}

// emit stores a completed transcription and broadcasts it. `language` is recorded
// in the broadcast so the UI can tell a French transmission from an English one;
// on a bilingual band that distinction matters, and a wrong-language transcript
// must never be silently indistinguishable from a right one.
func (s *transcriptionSink) emit(text string, at time.Time, language string) error {
	record := &sqlite.TranscriptionRecord{
		FrequencyID:      s.frequencyID,
		CreatedAt:        at,
		Content:          text,
		IsComplete:       true,
		IsProcessed:      false,
		ContentProcessed: "",
		Language:         language,
	}

	id, err := s.storage.StoreTranscription(record)
	if err != nil {
		return fmt.Errorf("failed to store transcription: %w", err)
	}

	if s.fileLogger != nil {
		if err := s.fileLogger.LogRaw(s.frequencyID, at, text); err != nil {
			s.logger.Error("Failed to write raw transcription to log file", Error(err))
		}
	}

	s.wsServer.Broadcast(&websocket.Message{
		Type: "transcription",
		Data: map[string]interface{}{
			"id":                id,
			"frequency_id":      s.frequencyID,
			"text":              text,
			"timestamp":         at,
			"is_complete":       true,
			"is_processed":      false,
			"content_processed": "",
			"language":          language,
		},
	})
	return nil
}
