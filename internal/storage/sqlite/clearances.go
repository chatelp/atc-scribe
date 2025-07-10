package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// ClearanceStorage handles storage of clearance records
type ClearanceStorage struct {
	db     *sql.DB
	logger *logger.Logger
}

// NewClearanceStorage creates a new SQLite clearance storage
func NewClearanceStorage(db *sql.DB, logger *logger.Logger) *ClearanceStorage {
	storage := &ClearanceStorage{
		db:     db,
		logger: logger.Named("sqlite-clearances"),
	}

	// Initialize database
	if err := storage.initDB(); err != nil {
		logger.Error("Failed to initialize clearance storage", Error(err))
	}

	return storage
}

// initDB initializes the database tables
func (s *ClearanceStorage) initDB() error {
	// Create clearances table
	_, err := s.db.Exec(`
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
		_, err = s.db.Exec(indexSQL)
		if err != nil {
			return fmt.Errorf("failed to create clearance index: %w", err)
		}
	}

	return nil
}

// StoreClearance stores a clearance record
func (s *ClearanceStorage) StoreClearance(record *ClearanceRecord) (int64, error) {
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
		record.Timestamp.Format(time.RFC3339),
		record.Status,
		record.CreatedAt.Format(time.RFC3339),
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

// GetClearancesByTimeRange returns clearances within a time range
func (s *ClearanceStorage) GetClearancesByTimeRange(startTime, endTime time.Time) ([]*ClearanceRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, transcription_id, callsign, clearance_type, clearance_text, runway, timestamp, status, created_at 
		FROM clearances 
		WHERE timestamp BETWEEN ? AND ? 
		ORDER BY timestamp DESC`,
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query clearances by time range: %w", err)
	}
	defer rows.Close()

	return s.scanClearanceRows(rows)
}

// GetClearancesByType returns clearances of a specific type
func (s *ClearanceStorage) GetClearancesByType(clearanceType string, limit int) ([]*ClearanceRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, transcription_id, callsign, clearance_type, clearance_text, runway, timestamp, status, created_at 
		FROM clearances 
		WHERE clearance_type = ? 
		ORDER BY timestamp DESC 
		LIMIT ?`,
		clearanceType, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query clearances by type: %w", err)
	}
	defer rows.Close()

	return s.scanClearanceRows(rows)
}

// UpdateClearanceStatus updates the status of a clearance (for Phase 2 compliance monitoring)
func (s *ClearanceStorage) UpdateClearanceStatus(id int64, status string) error {
	// Update record
	_, err := s.db.Exec(
		`UPDATE clearances
		SET status = ?
		WHERE id = ?`,
		status,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update clearance status: %w", err)
	}

	return nil
}

// GetRecentClearances returns recent clearances across all aircraft
func (s *ClearanceStorage) GetRecentClearances(limit int) ([]*ClearanceRecord, error) {
	// Query records
	rows, err := s.db.Query(
		`SELECT id, transcription_id, callsign, clearance_type, clearance_text, runway, timestamp, status, created_at 
		FROM clearances 
		ORDER BY timestamp DESC 
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent clearances: %w", err)
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
