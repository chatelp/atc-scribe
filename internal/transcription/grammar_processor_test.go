package transcription

import (
	"context"
	"database/sql"
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

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
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
