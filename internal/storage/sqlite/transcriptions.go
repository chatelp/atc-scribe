package sqlite

import (
	"database/sql"
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
			callsign_source TEXT
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
		"language":        "ALTER TABLE transcriptions ADD COLUMN language TEXT",
		"content_second":  "ALTER TABLE transcriptions ADD COLUMN content_second TEXT",
		"callsign_source": "ALTER TABLE transcriptions ADD COLUMN callsign_source TEXT",
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source 
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
		var contentProcessed, language, contentSecond, callsignSource sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if language.Valid {
			record.Language = language.String
		}
		if contentSecond.Valid {
			record.ContentSecond = contentSecond.String
		}
		if callsignSource.Valid {
			record.CallsignSource = callsignSource.String
		}
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source 
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
		var contentProcessed, language, contentSecond, callsignSource sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if language.Valid {
			record.Language = language.String
		}
		if contentSecond.Valid {
			record.ContentSecond = contentSecond.String
		}
		if callsignSource.Valid {
			record.CallsignSource = callsignSource.String
		}
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source 
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
		var contentProcessed, language, contentSecond, callsignSource sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if language.Valid {
			record.Language = language.String
		}
		if contentSecond.Valid {
			record.ContentSecond = contentSecond.String
		}
		if callsignSource.Valid {
			record.CallsignSource = callsignSource.String
		}
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source 
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
		var contentProcessed, language, contentSecond, callsignSource sql.NullString

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
			&language,
			&contentSecond,
			&callsignSource,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if language.Valid {
			record.Language = language.String
		}
		if contentSecond.Valid {
			record.ContentSecond = contentSecond.String
		}
		if callsignSource.Valid {
			record.CallsignSource = callsignSource.String
		}
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source 
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
		var contentProcessed, language, contentSecond, callsignSource sql.NullString

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
		if language.Valid {
			record.Language = language.String
		}
		if contentSecond.Valid {
			record.ContentSecond = contentSecond.String
		}
		if callsignSource.Valid {
			record.CallsignSource = callsignSource.String
		}
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source
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
		var contentProcessed, language, contentSecond, callsignSource sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if language.Valid {
			record.Language = language.String
		}
		if contentSecond.Valid {
			record.ContentSecond = contentSecond.String
		}
		if callsignSource.Valid {
			record.CallsignSource = callsignSource.String
		}
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
func (s *TranscriptionStorage) UpdateProcessedTranscription(id int64, contentProcessed string, speakerType string, callsign string, callsignSource string) error {
	lockSQLiteWrite()
	defer unlockSQLiteWrite()

	// Update record
	_, err := s.db.Exec(
		`UPDATE transcriptions
		SET content_processed = ?, is_processed = 1, speaker_type = ?, callsign = ?, callsign_source = ?
		WHERE id = ?`,
		contentProcessed,
		speakerType,
		callsign,
		callsignSource,
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
		`SELECT id, frequency_id, created_at, content, is_complete, is_processed, content_processed, speaker_type, callsign, language, content_second, callsign_source
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
		var contentProcessed, language, contentSecond, callsignSource sql.NullString

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
		); err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}

		// Parse created_at
		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable fields
		if language.Valid {
			record.Language = language.String
		}
		if contentSecond.Valid {
			record.ContentSecond = contentSecond.String
		}
		if callsignSource.Valid {
			record.CallsignSource = callsignSource.String
		}
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
