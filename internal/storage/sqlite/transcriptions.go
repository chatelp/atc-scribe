package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// Import logger functions
var (
	String = logger.String
	Error  = logger.Error
)

// TranscriptionRecord represents a transcription record in the database
type TranscriptionRecord struct {
	ID               int64     `json:"id"`
	FrequencyID      string    `json:"frequency_id"`
	CreatedAt        time.Time `json:"created_at"`
	Content          string    `json:"content"`
	IsComplete       bool      `json:"is_complete"`
	IsProcessed      bool      `json:"is_processed"`
	ContentProcessed string    `json:"content_processed"`
	SpeakerType      string    `json:"speaker_type,omitempty"`    // "ATC" or "PILOT"
	Callsign         string    `json:"callsign,omitempty"`        // Aircraft callsign if speaker is a pilot
	Language         string    `json:"language,omitempty"`        // as decoded: "en", "fr", empty when unknown
	ContentSecond    string    `json:"content_second,omitempty"`  // a second model on the same audio, when the gate opened
	CallsignSource   string    `json:"callsign_source,omitempty"` // which reading the callsign came from

	// The words that named the aircraft, kept when the match was made: [start,
	// end) offsets in UTF-16 code units, the unit a browser indexes strings in.
	// They point into the text displayed for the reading callsign_source names --
	// content_processed for the primary one, content_second for the French one.
	CallsignEvidence [][2]int `json:"callsign_evidence,omitempty"`
}

// TranscriptionStorage handles storage of transcription records
type TranscriptionStorage struct {
	db     *DB
	logger *logger.Logger
}

// NewTranscriptionStorage creates a new SQLite transcription storage
func NewTranscriptionStorage(db *DB, logger *logger.Logger) *TranscriptionStorage {
	storage := &TranscriptionStorage{
		db:     db,
		logger: logger.Named("sqlite-tx"),
	}

	// Initialize database
	if err := storage.initDB(); err != nil {
		logger.Error("Failed to initialize transcription storage", Error(err))
	}

	return storage
}

// initDB initializes the database tables
func (s *TranscriptionStorage) initDB() error {
	// Create transcriptions table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS transcriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			frequency_id TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			content TEXT NOT NULL,
			is_complete BOOLEAN NOT NULL,
			is_processed BOOLEAN NOT NULL,
			content_processed TEXT,
			speaker_type TEXT,
			callsign TEXT,
			language TEXT,
			content_second TEXT,
			callsign_source TEXT,
			callsign_evidence TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create transcriptions table: %w", err)
	}

	// The band is bilingual, so which language a transmission was decoded as is
	// a measurement in its own right -- it is how a French model gets compared
	// with an English one, and how a wrong-language transcript gets found at all.
	// Upstream's schema has no such column; databases written before this exist,
	// so add it where it is missing rather than requiring a fresh file.
	// content_second holds a second model's reading of the same audio, kept when
	// the sidecar's French gate opened; callsign_source says which reading the
	// callsign came from. Both exist so a disagreement stays inspectable: the
	// text a human reads always comes from the primary model, and an error can
	// still be traced to the model that produced it.
	for column, ddl := range map[string]string{
		"language":          "ALTER TABLE transcriptions ADD COLUMN language TEXT",
		"content_second":    "ALTER TABLE transcriptions ADD COLUMN content_second TEXT",
		"callsign_source":   "ALTER TABLE transcriptions ADD COLUMN callsign_source TEXT",
		"callsign_evidence": "ALTER TABLE transcriptions ADD COLUMN callsign_evidence TEXT",
	} {
		if _, err := s.db.Exec(ddl); err != nil &&
			!strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("failed to add transcriptions.%s: %w", column, err)
		}
	}

	// Create indexes
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_frequency_id ON transcriptions(frequency_id)`)
	if err != nil {
		return fmt.Errorf("failed to create frequency_id index: %w", err)
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_created_at ON transcriptions(created_at)`)
	if err != nil {
		return fmt.Errorf("failed to create created_at index: %w", err)
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_speaker_type ON transcriptions(speaker_type)`)
	if err != nil {
		return fmt.Errorf("failed to create speaker_type index: %w", err)
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_callsign ON transcriptions(callsign)`)
	if err != nil {
		return fmt.Errorf("failed to create callsign index: %w", err)
	}

	return nil
}

// transcriptionColumns is the one column list every reader selects, in the order
// readTranscriptions scans them. Seven functions used to repeat both. On 20/09 one
// of them gained three columns in its SELECT and not in its Scan: every call
// failed, and the aircraft card showed nothing for aircraft heard on the radio.
const transcriptionColumns = "id, frequency_id, created_at, content, is_complete, is_processed, " +
	"content_processed, speaker_type, callsign, language, content_second, callsign_source, callsign_evidence"

// readTranscriptions scans every row of a query that selected transcriptionColumns.
func readTranscriptions(rows *sql.Rows) ([]*TranscriptionRecord, error) {
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var contentProcessed, speakerType, callsign, language, contentSecond, callsignSource, evidence sql.NullString
		if err := rows.Scan(
			&record.ID,
			&record.FrequencyID,
			&createdAt,
			&record.Content,
			&record.IsComplete,
			&record.IsProcessed,
			&contentProcessed,
			&speakerType,
			&callsign,
			&language,
			&contentSecond,
			&callsignSource,
			&evidence,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}
		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		record.CreatedAt = t
		// A NULL column reads as "", which is what every field defaulted to.
		record.ContentProcessed = contentProcessed.String
		record.SpeakerType = speakerType.String
		record.Callsign = callsign.String
		record.Language = language.String
		record.ContentSecond = contentSecond.String
		record.CallsignSource = callsignSource.String
		if evidence.String != "" {
			// A malformed highlight costs the highlight, not the transcription.
			if err := json.Unmarshal([]byte(evidence.String), &record.CallsignEvidence); err != nil {
				record.CallsignEvidence = nil
			}
		}
		records = append(records, &record)
	}
	return records, rows.Err()
}

// StoreTranscription stores a transcription record
func (s *TranscriptionStorage) StoreTranscription(record *TranscriptionRecord) (int64, error) {
	lockSQLiteWrite()
	defer unlockSQLiteWrite()

	// Insert record
	result, err := s.db.Exec(
		`INSERT INTO transcriptions 
		(frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.FrequencyID,
		record.CreatedAt.Format(time.RFC3339),
		record.Content,
		record.IsComplete,
		record.IsProcessed,
		record.ContentProcessed,
		record.SpeakerType,
		record.Callsign,
		record.Language,
		record.ContentSecond,
		record.CallsignSource,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert transcription: %w", err)
	}

	// Get ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// GetTranscriptions returns all transcriptions with pagination
func (s *TranscriptionStorage) GetTranscriptions(limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT `+transcriptionColumns+`
		FROM transcriptions 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query transcriptions: %w", err)
	}
	defer rows.Close()

	return readTranscriptions(rows)
}

// GetTranscriptionsByFrequency returns transcriptions for a specific frequency
func (s *TranscriptionStorage) GetTranscriptionsByFrequency(frequencyID string, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT `+transcriptionColumns+`
		FROM transcriptions 
		WHERE frequency_id = ? 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`,
		frequencyID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query transcriptions by frequency: %w", err)
	}
	defer rows.Close()

	return readTranscriptions(rows)
}

// GetTranscriptionsByTimeRange returns transcriptions within a time range
func (s *TranscriptionStorage) GetTranscriptionsByTimeRange(startTime, endTime time.Time, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT `+transcriptionColumns+`
		FROM transcriptions 
		WHERE created_at BETWEEN ? AND ? 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`,
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query transcriptions by time range: %w", err)
	}
	defer rows.Close()

	return readTranscriptions(rows)
}

// GetTranscriptionsBySpeaker returns transcriptions by speaker type
func (s *TranscriptionStorage) GetTranscriptionsBySpeaker(speakerType string, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT `+transcriptionColumns+`
		FROM transcriptions 
		WHERE speaker_type = ? 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`,
		speakerType, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query transcriptions by speaker: %w", err)
	}
	defer rows.Close()

	return readTranscriptions(rows)
}

// GetTranscriptionsByCallsign returns transcriptions by aircraft callsign
func (s *TranscriptionStorage) GetTranscriptionsByCallsign(callsign string, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT `+transcriptionColumns+`
		FROM transcriptions 
		WHERE callsign = ? 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`,
		callsign, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query transcriptions by callsign: %w", err)
	}
	defer rows.Close()

	return readTranscriptions(rows)
}

// GetUnprocessedTranscriptions retrieves a batch of unprocessed transcriptions
func (s *TranscriptionStorage) GetUnprocessedTranscriptions(batchSize int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT `+transcriptionColumns+`
		FROM transcriptions
		WHERE is_complete = 1 AND is_processed = 0
		ORDER BY created_at ASC
		LIMIT ?`,
		batchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed transcriptions: %w", err)
	}
	defer rows.Close()

	return readTranscriptions(rows)
}

// UpdateProcessedTranscription updates a transcription with processed content
func (s *TranscriptionStorage) UpdateProcessedTranscription(id int64, contentProcessed string, speakerType string, callsign string, callsignSource string) error {
	return s.UpdateMatchedTranscription(id, contentProcessed, speakerType, callsign, callsignSource, nil)
}

// UpdateMatchedTranscription is UpdateProcessedTranscription with the words that
// named the aircraft, written in the same statement: a callsign stored without
// its evidence, or the reverse, would highlight words that named nothing.
func (s *TranscriptionStorage) UpdateMatchedTranscription(id int64, contentProcessed, speakerType, callsign, callsignSource string, evidence [][2]int) error {
	var ev sql.NullString
	if len(evidence) > 0 {
		b, err := json.Marshal(evidence)
		if err != nil {
			return fmt.Errorf("encode callsign evidence: %w", err)
		}
		ev = sql.NullString{String: string(b), Valid: true}
	}

	lockSQLiteWrite()
	defer unlockSQLiteWrite()

	// Update record
	_, err := s.db.Exec(
		`UPDATE transcriptions
		SET content_processed = ?, is_processed = 1, speaker_type = ?, callsign = ?, callsign_source = ?, callsign_evidence = ?
		WHERE id = ?`,
		contentProcessed,
		speakerType,
		callsign,
		callsignSource,
		ev,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update processed transcription: %w", err)
	}

	return nil
}

// GetLastProcessedTranscriptions retrieves the last N processed transcriptions for a given frequency
func (s *TranscriptionStorage) GetLastProcessedTranscriptions(frequencyID string, limit int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT `+transcriptionColumns+`
		FROM transcriptions
		WHERE frequency_id = ? AND is_processed = 1
		ORDER BY created_at DESC
		LIMIT ?`,
		frequencyID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query last processed transcriptions: %w", err)
	}
	defer rows.Close()

	return readTranscriptions(rows)
}
