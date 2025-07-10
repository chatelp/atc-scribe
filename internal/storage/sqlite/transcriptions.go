package sqlite

import (
	"database/sql"
	"fmt"
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
	SpeakerType      string    `json:"speaker_type,omitempty"` // "ATC" or "PILOT"
	Callsign         string    `json:"callsign,omitempty"`     // Aircraft callsign if speaker is a pilot
}

// TranscriptionStorage handles storage of transcription records
type TranscriptionStorage struct {
	db     *sql.DB
	logger *logger.Logger
}

// NewTranscriptionStorage creates a new SQLite transcription storage
func NewTranscriptionStorage(db *sql.DB, logger *logger.Logger) *TranscriptionStorage {
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
			callsign TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create transcriptions table: %w", err)
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

// StoreTranscription stores a transcription record
func (s *TranscriptionStorage) StoreTranscription(record *TranscriptionRecord) (int64, error) {
	// Insert record
	result, err := s.db.Exec(
		`INSERT INTO transcriptions 
		(frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.FrequencyID,
		record.CreatedAt.Format(time.RFC3339),
		record.Content,
		record.IsComplete,
		record.IsProcessed,
		record.ContentProcessed,
		record.SpeakerType,
		record.Callsign,
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign 
		FROM transcriptions 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query transcriptions: %w", err)
	}
	defer rows.Close()

	// Parse records
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var speakerType, callsign sql.NullString
		var contentProcessed sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if contentProcessed.Valid {
			record.ContentProcessed = contentProcessed.String
		}
		if speakerType.Valid {
			record.SpeakerType = speakerType.String
		}
		if callsign.Valid {
			record.Callsign = callsign.String
		}

		records = append(records, &record)
	}

	return records, nil
}

// GetTranscriptionsByFrequency returns transcriptions for a specific frequency
func (s *TranscriptionStorage) GetTranscriptionsByFrequency(frequencyID string, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign 
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

	// Parse records
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var speakerType, callsign sql.NullString
		var contentProcessed sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if contentProcessed.Valid {
			record.ContentProcessed = contentProcessed.String
		}
		if speakerType.Valid {
			record.SpeakerType = speakerType.String
		}
		if callsign.Valid {
			record.Callsign = callsign.String
		}

		records = append(records, &record)
	}

	return records, nil
}

// GetTranscriptionsByTimeRange returns transcriptions within a time range
func (s *TranscriptionStorage) GetTranscriptionsByTimeRange(startTime, endTime time.Time, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign 
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

	// Parse records
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var speakerType, callsign sql.NullString
		var contentProcessed sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if contentProcessed.Valid {
			record.ContentProcessed = contentProcessed.String
		}
		if speakerType.Valid {
			record.SpeakerType = speakerType.String
		}
		if callsign.Valid {
			record.Callsign = callsign.String
		}

		records = append(records, &record)
	}

	return records, nil
}

// GetTranscriptionsBySpeaker returns transcriptions by speaker type
func (s *TranscriptionStorage) GetTranscriptionsBySpeaker(speakerType string, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign 
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

	// Parse records
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var speakerTypeDB, callsign sql.NullString
		var contentProcessed sql.NullString

		if err := rows.Scan(
			&record.ID,
			&record.FrequencyID,
			&createdAt,
			&record.Content,
			&record.IsComplete,
			&record.IsProcessed,
			&contentProcessed,
			&speakerTypeDB,
			&callsign,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if contentProcessed.Valid {
			record.ContentProcessed = contentProcessed.String
		}
		if speakerTypeDB.Valid {
			record.SpeakerType = speakerTypeDB.String
		}
		if callsign.Valid {
			record.Callsign = callsign.String
		}

		records = append(records, &record)
	}

	return records, nil
}

// GetTranscriptionsByCallsign returns transcriptions by aircraft callsign
func (s *TranscriptionStorage) GetTranscriptionsByCallsign(callsign string, limit, offset int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign 
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

	// Parse records
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var speakerType, callsignDB sql.NullString
		var contentProcessed sql.NullString

		if err := rows.Scan(
			&record.ID,
			&record.FrequencyID,
			&createdAt,
			&record.Content,
			&record.IsComplete,
			&record.IsProcessed,
			&contentProcessed,
			&speakerType,
			&callsignDB,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if contentProcessed.Valid {
			record.ContentProcessed = contentProcessed.String
		}
		if speakerType.Valid {
			record.SpeakerType = speakerType.String
		}
		if callsignDB.Valid {
			record.Callsign = callsignDB.String
		}

		records = append(records, &record)
	}

	return records, nil
}

// GetUnprocessedTranscriptions retrieves a batch of unprocessed transcriptions
func (s *TranscriptionStorage) GetUnprocessedTranscriptions(batchSize int) ([]*TranscriptionRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign
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

	// Parse records
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var speakerType, callsign sql.NullString
		var contentProcessed sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if contentProcessed.Valid {
			record.ContentProcessed = contentProcessed.String
		}
		if speakerType.Valid {
			record.SpeakerType = speakerType.String
		}
		if callsign.Valid {
			record.Callsign = callsign.String
		}

		records = append(records, &record)
	}

	return records, nil
}

// UpdateProcessedTranscription updates a transcription with processed content
func (s *TranscriptionStorage) UpdateProcessedTranscription(id int64, contentProcessed string, speakerType string, callsign string) error {
	// Update record
	_, err := s.db.Exec(
		`UPDATE transcriptions
		SET content_processed = ?, is_processed = 1, speaker_type = ?, callsign = ?
		WHERE id = ?`,
		contentProcessed,
		speakerType,
		callsign,
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign
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

	// Parse records
	var records []*TranscriptionRecord
	for rows.Next() {
		var record TranscriptionRecord
		var createdAt string
		var speakerType, callsign sql.NullString
		var contentProcessed sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if contentProcessed.Valid {
			record.ContentProcessed = contentProcessed.String
		}
		if speakerType.Valid {
			record.SpeakerType = speakerType.String
		}
		if callsign.Valid {
			record.Callsign = callsign.String
		}

		records = append(records, &record)
	}

	return records, nil
}
