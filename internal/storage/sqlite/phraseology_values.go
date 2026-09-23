package sqlite

import (
	"fmt"
	"strings"
	"time"
)

// PhraseologyValue is one fact the grammar recovered from a transmission: a level,
// a heading, a runway, a squawk.
//
// These are stored in a table of their own rather than as a column on
// transcriptions. Upstream creates that table with CREATE TABLE IF NOT EXISTS and
// never migrates it, so adding a column would leave every existing database one
// schema behind with nothing to notice it. A separate table appears on its own for
// old and new databases alike, and it stays out of the way of a rebase.
//
// One row per value rather than a JSON blob, because the useful questions are
// relational: every level ever given to one aircraft, every runway heard tonight.
type PhraseologyValue struct {
	ID              int64     `json:"id"`
	TranscriptionID int64     `json:"transcription_id"`
	Callsign        string    `json:"callsign,omitempty"`
	Role            string    `json:"role"`   // flight_level, altitude, heading, speed, frequency, runway, qnh, squawk
	Digits          string    `json:"digits"` // as spoken, e.g. "350"
	Text            string    `json:"text"`   // normalised for display, e.g. "FL350", "27L"
	CreatedAt       time.Time `json:"created_at"`
}

// PhraseologyStorage holds what the grammar extracted, next to the transcriptions
// it came from.
type PhraseologyStorage struct {
	db *DB
}

func NewPhraseologyStorage(db *DB) *PhraseologyStorage {
	return &PhraseologyStorage{db: db}
}

// createPhraseologySchema runs at every open of a daily file, from initDatabase.
func createPhraseologySchema(db execer) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS phraseology_values (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transcription_id INTEGER NOT NULL,
			callsign TEXT,
			role TEXT NOT NULL,
			digits TEXT NOT NULL,
			text TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (transcription_id) REFERENCES transcriptions(id)
		)`); err != nil {
		return fmt.Errorf("failed to create phraseology_values table: %w", err)
	}
	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS idx_phv_transcription ON phraseology_values(transcription_id)`,
		`CREATE INDEX IF NOT EXISTS idx_phv_callsign ON phraseology_values(callsign)`,
		`CREATE INDEX IF NOT EXISTS idx_phv_role ON phraseology_values(role)`,
	} {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create phraseology index: %w", err)
		}
	}
	return nil
}

// StoreValues records everything one transmission yielded, in a single statement.
func (s *PhraseologyStorage) StoreValues(values []PhraseologyValue) error {
	if len(values) == 0 {
		return nil
	}
	lockSQLiteWrite()
	defer unlockSQLiteWrite()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO phraseology_values
		(transcription_id, callsign, role, digits, text, created_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, v := range values {
		if _, err := stmt.Exec(v.TranscriptionID, v.Callsign, v.Role, v.Digits, v.Text, v.CreatedAt); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to store phraseology value: %w", err)
		}
	}
	return tx.Commit()
}

// ValuesByTranscription returns the values of the given transcriptions, keyed by
// transcription id. One query for the whole page rather than one per row.
func (s *PhraseologyStorage) ValuesByTranscription(ids []int64) (map[int64][]PhraseologyValue, error) {
	out := map[int64][]PhraseologyValue{}
	if len(ids) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT id, transcription_id, COALESCE(callsign,''), role, digits, text, created_at
		FROM phraseology_values WHERE transcription_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY id`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to read phraseology values: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var v PhraseologyValue
		var at string
		if err := rows.Scan(&v.ID, &v.TranscriptionID, &v.Callsign, &v.Role, &v.Digits, &v.Text, &at); err != nil {
			return nil, err
		}
		if t, err := parseTime(at); err == nil {
			v.CreatedAt = t
		}
		out[v.TranscriptionID] = append(out[v.TranscriptionID], v)
	}
	return out, rows.Err()
}

// DBOf exposes the connection a transcription storage was built on, so a
// phraseology storage can be opened on the same daily database without threading
// a fourth storage through three upstream constructors. They are always the same
// file: the values belong to the transcriptions they were parsed from.
func DBOf(s *TranscriptionStorage) *DB { return s.db }

// TranscriptionsWithoutValues returns transcriptions that have been annotated but
// whose values were never recorded, oldest first.
//
// It exists because the value table arrived after the transcriptions did: without
// it, every row stored before this feature would show a transmission with no facts
// beside it, for ever, and a restart would never repair it.
func (s *PhraseologyStorage) TranscriptionsWithoutValues(limit int) ([]TranscriptionRecord, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.frequency_id, t.created_at, t.content, COALESCE(t.callsign, '')
		FROM transcriptions t
		LEFT JOIN phraseology_values v ON v.transcription_id = t.id
		WHERE t.is_processed = 1 AND v.id IS NULL
		GROUP BY t.id
		ORDER BY t.id
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find transcriptions without values: %w", err)
	}
	defer rows.Close()

	var out []TranscriptionRecord
	for rows.Next() {
		var r TranscriptionRecord
		var at string
		if err := rows.Scan(&r.ID, &r.FrequencyID, &at, &r.Content, &r.Callsign); err != nil {
			return nil, err
		}
		if t, err := parseTime(at); err == nil {
			r.CreatedAt = t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
