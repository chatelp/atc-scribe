package sqlite

import "time"

// ClearanceRecord represents a clearance extracted from transcriptions
type ClearanceRecord struct {
	ID              int64     `json:"id"`
	TranscriptionID int64     `json:"transcription_id"`
	Callsign        string    `json:"callsign"`
	ClearanceType   string    `json:"clearance_type"` // "takeoff" or "landing"
	ClearanceText   string    `json:"clearance_text"`
	Runway          string    `json:"runway,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	Status          string    `json:"status"` // "issued", "complied", "deviation"
	CreatedAt       time.Time `json:"created_at"`
}
