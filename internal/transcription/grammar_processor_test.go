package transcription

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/transcription/phraseology"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// fixedFleet is a sky that does not move, so a test can state exactly which
// aircraft were in the air when something was said.
type fixedFleet []phraseology.Aircraft

func (f fixedFleet) Fleet() []phraseology.Aircraft { return f }

func newTestProcessor(t *testing.T, fleet FleetProvider) (*GrammarProcessor, *sqlite.TranscriptionStorage, *sqlite.ClearanceStorage) {
	t.Helper()

	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"), log)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	txStore := sqlite.NewTranscriptionStorage(db, log)
	clStore := sqlite.NewClearanceStorage(db, log)

	proc, err := NewGrammarProcessor(
		context.Background(),
		txStore,
		clStore,
		websocket.NewServer(log),
		fleet,
		PostProcessingConfig{
			Enabled:         true,
			Backend:         BackendLocal,
			AirlinesDatPath: filepath.Join("..", "..", "assets", "airlines.dat"),
			BatchSize:       20,
			IntervalSeconds: 1,
		},
		log,
	)
	if err != nil {
		t.Fatalf("NewGrammarProcessor: %v", err)
	}
	return proc, txStore, clStore
}

func store(t *testing.T, s *sqlite.TranscriptionStorage, text string) int64 {
	t.Helper()
	id, err := s.StoreTranscription(&sqlite.TranscriptionRecord{
		FrequencyID: "124350",
		CreatedAt:   time.Now().UTC(),
		Content:     text,
		IsComplete:  true,
	})
	if err != nil {
		t.Fatalf("StoreTranscription: %v", err)
	}
	return id
}

func reread(t *testing.T, s *sqlite.TranscriptionStorage, id int64) *sqlite.TranscriptionRecord {
	t.Helper()
	records, err := s.GetTranscriptions(100, 0)
	if err != nil {
		t.Fatalf("GetTranscriptions: %v", err)
	}
	for _, r := range records {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("transcription %d disappeared", id)
	return nil
}

// A transmission naming an aircraft the receiver can see should end up attached
// to it, with the ADS-B callsign rather than the spoken one — that is what the
// aircraft API joins on.
func TestGrammarProcessorAttachesTransmissionToVisibleAircraft(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR1081", Hex: "39c1a2", AltitudeFt: 10000, Phase: "ARR"},
		{Callsign: "BAW572", Hex: "400a1b", AltitudeFt: 37000, Phase: "CRZ"},
	})

	id := store(t, txStore, "air france one zero eight one descend flight level one zero zero")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	got := reread(t, txStore, id)
	if got.Callsign != "AFR1081" {
		t.Errorf("callsign = %q, want AFR1081", got.Callsign)
	}
	if got.SpeakerType != string(phraseology.SpeakerATC) {
		t.Errorf("speaker_type = %q, want ATC", got.SpeakerType)
	}
	if !got.IsProcessed {
		t.Error("record was left unprocessed, so it would be retried forever")
	}
}

// An aircraft that is not in the sky must not be invented. This is the case that
// matters most: a matcher that always answers is worse than one that refuses.
func TestGrammarProcessorRefusesAnAbsentAircraft(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "BAW572", Hex: "400a1b", AltitudeFt: 37000, Phase: "CRZ"},
	})

	id := store(t, txStore, "air france one zero eight one descend flight level one zero zero")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	got := reread(t, txStore, id)
	if got.Callsign != "" {
		t.Errorf("callsign = %q, want none: AFR1081 was not in the sky", got.Callsign)
	}
	if !got.IsProcessed {
		t.Error("an unmatched record must still be marked processed")
	}
}

// A clearance is filed under the ADS-B callsign, which is the whole point: the
// aircraft API looks clearances up by aircraft.Flight.
func TestGrammarProcessorFilesClearanceUnderADSBCallsign(t *testing.T) {
	proc, txStore, clStore := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR1081", Hex: "39c1a2", AltitudeFt: 3000, Phase: "ARR"},
	})

	store(t, txStore, "air france one zero eight one cleared to land runway two seven right")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	clearances, err := clStore.GetClearancesByCallsign("AFR1081", 10)
	if err != nil {
		t.Fatalf("GetClearancesByCallsign: %v", err)
	}
	if len(clearances) != 1 {
		t.Fatalf("got %d clearances, want 1", len(clearances))
	}
	if clearances[0].ClearanceType != "landing" {
		t.Errorf("clearance type = %q, want landing", clearances[0].ClearanceType)
	}
	if clearances[0].Runway != "27R" {
		t.Errorf("runway = %q, want 27R", clearances[0].Runway)
	}
}

// With no ADS-B at all the grammar still has work to do: it can say who spoke and
// normalise the text. It must not fall over, and it must not stall the queue.
func TestGrammarProcessorWorksWithoutADSB(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, nil)

	id := store(t, txStore, "descend flight level one zero zero")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	got := reread(t, txStore, id)
	if !got.IsProcessed {
		t.Error("record left unprocessed with no ADS-B available")
	}
	if got.ContentProcessed == "" {
		t.Error("content_processed is empty, which the UI renders as a blank line")
	}
}

// storeTwo records a transmission with two readings of the same audio, as the
// sidecar produces when its French gate opens.
func storeTwo(t *testing.T, s *sqlite.TranscriptionStorage, primary, second string) int64 {
	t.Helper()
	id, err := s.StoreTranscription(&sqlite.TranscriptionRecord{
		FrequencyID:   "124350",
		CreatedAt:     time.Now().UTC(),
		Content:       primary,
		ContentSecond: second,
		IsComplete:    true,
	})
	if err != nil {
		t.Fatalf("StoreTranscription: %v", err)
	}
	return id
}

// The case the whole second reading exists for, taken from the 15/09 capture:
// the English model heard "zero seven two" and the French one "062". ADS-B had
// AFR062, so without the second reading this transmission stays anonymous.
func TestSecondReadingRescuesAMatchThePrimaryMissed(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR062", Hex: "39a1b2", AltitudeFt: 24000, Phase: "CRZ"},
	})

	id := storeTwo(t, txStore,
		"rehear les niveaux unite nine zero air france zero seven two",
		"enchire les niveaux unite ninety air france zero six two")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	r := reread(t, txStore, id)
	if r.Callsign != "AFR062" {
		t.Errorf("callsign: got %q, want AFR062", r.Callsign)
	}
	if r.CallsignSource != "fr" {
		t.Errorf("source: got %q, want fr", r.CallsignSource)
	}
	// The text a reader sees still comes from the primary model, so the corpus
	// stays comparable from end to end.
	if r.Content != "rehear les niveaux unite nine zero air france zero seven two" {
		t.Errorf("the stored text should be the primary reading, got %q", r.Content)
	}
}

func TestPrimaryReadingWinsWhenBothMatchTheSameAircraft(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR1081", Hex: "39c1a2", AltitudeFt: 10000, Phase: "ARR"},
	})

	id := storeTwo(t, txStore,
		"air france one zero eight one descend flight level one zero zero",
		"air france one zero eight one descendez niveau cent")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	r := reread(t, txStore, id)
	if r.Callsign != "AFR1081" {
		t.Errorf("callsign: got %q, want AFR1081", r.Callsign)
	}
	if r.CallsignSource != "en" {
		t.Errorf("agreement must be recorded as the primary's, got %q", r.CallsignSource)
	}
}

// Measured on the 15/09 capture, the two readings never named different
// aircraft inside the gate -- 8 agreements out of 8. But 0 of 8 bounds nothing,
// so the rule exists and is tested: the primary wins, because it runs on every
// transmission and its 72% precision is the one that was measured.
func TestPrimaryWinsAndTheDisagreementIsRecorded(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR1081", Hex: "39c1a2", AltitudeFt: 10000, Phase: "ARR"},
		{Callsign: "AFR2043", Hex: "39d4e5", AltitudeFt: 12000, Phase: "ARR"},
	})

	id := storeTwo(t, txStore,
		"air france one zero eight one descend flight level one zero zero",
		"air france two zero four three descendez niveau cent")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	r := reread(t, txStore, id)
	if r.Callsign != "AFR1081" {
		t.Errorf("the primary reading must win: got %q, want AFR1081", r.Callsign)
	}
	if r.CallsignSource != "en>fr" {
		t.Errorf("a disagreement must be recorded so it can be remeasured later, got %q", r.CallsignSource)
	}
}

// With no gate, nothing changes: this is the 87.5% of transmissions.
func TestASingleReadingBehavesExactlyAsBefore(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR1081", Hex: "39c1a2", AltitudeFt: 10000, Phase: "ARR"},
	})

	id := store(t, txStore, "air france one zero eight one descend flight level one zero zero")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}

	r := reread(t, txStore, id)
	if r.Callsign != "AFR1081" || r.CallsignSource != "en" {
		t.Errorf("got callsign %q from %q, want AFR1081 from en", r.Callsign, r.CallsignSource)
	}
	if r.ContentSecond != "" {
		t.Errorf("no gate means no second reading, got %q", r.ContentSecond)
	}
}
