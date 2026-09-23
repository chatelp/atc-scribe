package sqlite

import (
	"testing"
	"time"
)

// The aircraft card asks for the transmissions matched to one callsign. On
// 20/09 three columns were added to this query and not to its read, so every
// call failed -- and the card, which asks again on each position update,
// blinked between "loading" and "nothing matched" for exactly the aircraft the
// map marked as heard on the radio.
func TestTranscriptionsByCallsignAreReadBack(t *testing.T) {
	log := testLog(t)
	aircraft, err := NewAircraftStorage(DailyPath(t.TempDir(), time.Now()), log)
	if err != nil {
		t.Fatalf("NewAircraftStorage: %v", err)
	}
	s := NewTranscriptionStorage(aircraft.GetDB(), log)

	heard := &TranscriptionRecord{
		FrequencyID:    "aero-melange",
		CreatedAt:      time.Now().UTC().Truncate(time.Second),
		Content:        "air france three two uniform november descend flight level one two zero",
		IsComplete:     true,
		IsProcessed:    true,
		SpeakerType:    "ATC",
		Callsign:       "AFR32UN",
		Language:       "en",
		ContentSecond:  "air france trente-deux uniform novembre",
		CallsignSource: "second",
	}
	if _, err := s.StoreTranscription(heard); err != nil {
		t.Fatalf("StoreTranscription: %v", err)
	}
	other := *heard
	other.Callsign = "HGO772"
	if _, err := s.StoreTranscription(&other); err != nil {
		t.Fatalf("StoreTranscription: %v", err)
	}

	got, err := s.GetTranscriptionsByCallsign("AFR32UN", 50, 0)
	if err != nil {
		t.Fatalf("GetTranscriptionsByCallsign: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d transcriptions for AFR32UN, want 1", len(got))
	}
	r := got[0]
	for field, pair := range map[string][2]string{
		"callsign":        {r.Callsign, heard.Callsign},
		"language":        {r.Language, heard.Language},
		"content_second":  {r.ContentSecond, heard.ContentSecond},
		"callsign_source": {r.CallsignSource, heard.CallsignSource},
		"content":         {r.Content, heard.Content},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s: got %q, want %q", field, pair[0], pair[1])
		}
	}
}
