package sqlite

import (
	"fmt"
	"strings"
	"time"
)

// VoiceSummary is what the radio has said about one aircraft.
//
// It exists because the matching in internal/transcription/phraseology fills the
// callsign column of a transcription, and nothing downstream could see it: the
// map had no way to tell an aircraft the controller has just addressed from the
// three hundred others in the sky.
type VoiceSummary struct {
	Transmissions int       `json:"transmissions"`
	LastHeard     time.Time `json:"last_heard"`
	LastText      string    `json:"last_text"`
}

// VoiceSummaries returns, per ADS-B callsign, what voice has attached to it.
//
// One grouped query rather than one query per aircraft. The aircraft API already
// fetches clearances in a loop over three hundred targets, which is three hundred
// round trips a refresh; there was no reason to add a second such loop, and the
// callsign index makes the grouped form cheap.
func (s *TranscriptionStorage) VoiceSummaries() (map[string]VoiceSummary, error) {
	rows, err := s.db.Query(`
		SELECT t.callsign, COUNT(*), MAX(t.created_at)
		FROM transcriptions t
		WHERE t.callsign IS NOT NULL AND TRIM(t.callsign) != ''
		GROUP BY t.callsign`)
	if err != nil {
		return nil, fmt.Errorf("failed to summarise voice by callsign: %w", err)
	}
	defer rows.Close()

	out := map[string]VoiceSummary{}
	for rows.Next() {
		var callsign, last string
		var n int
		if err := rows.Scan(&callsign, &n, &last); err != nil {
			return nil, err
		}
		v := VoiceSummary{Transmissions: n}
		if t, err := parseTime(last); err == nil {
			v.LastHeard = t
		}
		out[strings.TrimSpace(callsign)] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// The latest text, for the aircraft that have one. Fetched separately so the
	// grouped count above stays a single scan of the index.
	texts, err := s.db.Query(`
		SELECT callsign, content, created_at FROM transcriptions
		WHERE callsign IS NOT NULL AND TRIM(callsign) != ''
		ORDER BY created_at ASC`)
	if err != nil {
		return out, nil // the counts alone are still useful
	}
	defer texts.Close()
	for texts.Next() {
		var callsign, content, at string
		if err := texts.Scan(&callsign, &content, &at); err != nil {
			break
		}
		callsign = strings.TrimSpace(callsign)
		if v, ok := out[callsign]; ok {
			v.LastText = content // rows are ascending, so the last write wins
			out[callsign] = v
		}
	}
	return out, nil
}

// parseTime accepts the shapes SQLite hands back for a TIMESTAMP column.
func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", s)
}
