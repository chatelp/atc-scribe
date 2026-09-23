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

// The two readers the page uses -- the frequency history and the aircraft card
// -- carry the words that named the aircraft, and a damaged value costs the
// highlight, never the transcription.
func TestCallsignEvidenceIsReadBackByThePageReaders(t *testing.T) {
	log := testLog(t)
	aircraft, err := NewAircraftStorage(DailyPath(t.TempDir(), time.Now()), log)
	if err != nil {
		t.Fatalf("NewAircraftStorage: %v", err)
	}
	s := NewTranscriptionStorage(aircraft.GetDB(), log)

	id, err := s.StoreTranscription(&TranscriptionRecord{
		FrequencyID: "aero-melange", CreatedAt: time.Now().UTC().Truncate(time.Second),
		Content: "air france one zero eight one", IsComplete: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]int{{0, 10}, {11, 29}}
	if err := s.UpdateMatchedTranscription(id, "air france one zero eight one", "ATC", "AFR1081", "en", want); err != nil {
		t.Fatalf("UpdateMatchedTranscription: %v", err)
	}
	byCS, err := s.GetTranscriptionsByCallsign("AFR1081", 10, 0)
	if err != nil || len(byCS) != 1 {
		t.Fatalf("by callsign: %v, %d records", err, len(byCS))
	}
	byFreq, err := s.GetTranscriptionsByFrequency("aero-melange", 10, 0)
	if err != nil || len(byFreq) != 1 {
		t.Fatalf("by frequency: %v, %d records", err, len(byFreq))
	}
	for name, r := range map[string]*TranscriptionRecord{"by callsign": byCS[0], "by frequency": byFreq[0]} {
		if len(r.CallsignEvidence) != 2 || r.CallsignEvidence[0] != want[0] || r.CallsignEvidence[1] != want[1] {
			t.Errorf("%s: evidence %v, want %v", name, r.CallsignEvidence, want)
		}
	}

	// Damaged by hand: the transcription must still be read.
	if _, err := s.db.Exec(`UPDATE transcriptions SET callsign_evidence = 'not json' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	again, err := s.GetTranscriptionsByCallsign("AFR1081", 10, 0)
	if err != nil || len(again) != 1 {
		t.Fatalf("a damaged highlight made the transcription unreadable: %v, %d records", err, len(again))
	}
	if again[0].CallsignEvidence != nil {
		t.Errorf("damaged evidence should read as none, got %v", again[0].CallsignEvidence)
	}
}
