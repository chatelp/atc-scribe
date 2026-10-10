package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// ClearanceStorage handles storage of clearance records
type ClearanceStorage struct {
	db     *DB
	logger *logger.Logger
}

// NewClearanceStorage creates a new SQLite clearance storage
func NewClearanceStorage(db *DB, logger *logger.Logger) *ClearanceStorage {
	return &ClearanceStorage{
		db:     db,
		logger: logger.Named("sqlite-clearances"),
	}
}

// createClearancesSchema runs at every open of a daily file, from initDatabase.
func createClearancesSchema(db execer) error {
	// Create clearances table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS clearances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transcription_id INTEGER NOT NULL,
			callsign TEXT NOT NULL,
			clearance_type TEXT NOT NULL,
			clearance_text TEXT NOT NULL,
			runway TEXT,
			timestamp TIMESTAMP NOT NULL,
			status TEXT NOT NULL DEFAULT 'issued',
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (transcription_id) REFERENCES transcriptions(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create clearances table: %w", err)
	}

	// Create indexes for performance
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_clearances_callsign ON clearances(callsign)`,
		`CREATE INDEX IF NOT EXISTS idx_clearances_timestamp ON clearances(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_clearances_type ON clearances(clearance_type)`,
		`CREATE INDEX IF NOT EXISTS idx_clearances_status ON clearances(status)`,
		`CREATE INDEX IF NOT EXISTS idx_clearances_transcription_id ON clearances(transcription_id)`,
	}

	for _, indexSQL := range indexes {
		_, err = db.Exec(indexSQL)
		if err != nil {
			return fmt.Errorf("failed to create clearance index: %w", err)
		}
	}

	// As for transcriptions: both times are compared as text and were kept in
	// the host's zone until 10/10; converted to UTC once, when the file is opened.
	for _, col := range []string{"created_at", "timestamp"} {
		if _, err := db.Exec(`UPDATE clearances SET ` + col + ` = strftime('%Y-%m-%dT%H:%M:%SZ', ` + col + `)
			WHERE ` + col + ` NOT LIKE '%Z' AND strftime('%Y-%m-%dT%H:%M:%SZ', ` + col + `) IS NOT NULL`); err != nil {
			return fmt.Errorf("failed to convert clearances.%s to UTC: %w", col, err)
		}
	}
	return nil
}

// StoreClearance stores a clearance record
func (s *ClearanceStorage) StoreClearance(record *ClearanceRecord) (int64, error) {
	lockSQLiteWrite()
	defer unlockSQLiteWrite()

	// Insert record
	result, err := s.db.Exec(
		`INSERT INTO clearances 
		(transcription_id, callsign, clearance_type, clearance_text, runway, timestamp, status, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.TranscriptionID,
		record.Callsign,
		record.ClearanceType,
		record.ClearanceText,
		record.Runway,
		record.Timestamp.UTC().Format(time.RFC3339),
		record.Status,
		record.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert clearance: %w", err)
	}

	// Get ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// GetClearancesByCallsign returns clearances for a specific aircraft callsign
func (s *ClearanceStorage) GetClearancesByCallsign(callsign string, limit int) ([]*ClearanceRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, transcription_id, callsign, clearance_type, clearance_text, runway, timestamp, status, created_at 
		FROM clearances 
		WHERE callsign = ? 
		ORDER BY timestamp DESC 
		LIMIT ?`,
		callsign, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query clearances by callsign: %w", err)
	}
	defer rows.Close()

	return s.scanClearanceRows(rows)
}

// scanClearanceRows scans database rows into ClearanceRecord structs
func (s *ClearanceStorage) scanClearanceRows(rows *sql.Rows) ([]*ClearanceRecord, error) {
	var records []*ClearanceRecord
	for rows.Next() {
		var record ClearanceRecord
		var timestamp, createdAt string
		var runway sql.NullString

		if err := rows.Scan(
			&record.ID,
			&record.TranscriptionID,
			&record.Callsign,
			&record.ClearanceType,
			&record.ClearanceText,
			&runway,
			&timestamp,
			&record.Status,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan clearance: %w", err)
		}

		// Parse timestamps
		var err error
		record.Timestamp, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}

		record.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		// Handle nullable runway field
		if runway.Valid {
			record.Runway = runway.String
		}

		records = append(records, &record)
	}

	return records, nil
}
