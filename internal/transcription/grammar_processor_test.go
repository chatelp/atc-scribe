package transcription

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unicode/utf16"

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

// highlighted cuts out of s the pieces the stored evidence points at, indexing
// in UTF-16 code units as the browser does.
func highlighted(s string, spans [][2]int) []string {
	u := utf16.Encode([]rune(s))
	var out []string
	for _, sp := range spans {
		if sp[0] < 0 || sp[1] > len(u) || sp[0] >= sp[1] {
			out = append(out, "<out of range>")
			continue
		}
		out = append(out, string(utf16.Decode(u[sp[0]:sp[1]])))
	}
	return out
}

// What the page highlights is read back from the database, not recomputed: the
// words that named the aircraft, placed in the processed text it displays.
func TestTheStoredEvidencePointsAtTheDisplayedWords(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR1081", Hex: "39c1a2", AltitudeFt: 10000, Phase: "ARR"},
	})
	id := store(t, txStore, "air france one zero eight one descend flight level one zero zero")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	r := reread(t, txStore, id)
	if r.CallsignSource != "en" {
		t.Fatalf("source: got %q, want en", r.CallsignSource)
	}
	got := highlighted(r.ContentProcessed, r.CallsignEvidence)
	if want := []string{"air france", "one zero eight one"}; !reflect.DeepEqual(got, want) {
		t.Errorf("stored evidence highlights %q in %q, want %q", got, r.ContentProcessed, want)
	}
}

// When the second reading named the aircraft, its words are in that reading --
// which is the text that has to be shown for the highlight to mean anything.
func TestEvidenceFromTheSecondReadingPointsIntoIt(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "AFR062", Hex: "39a1b2", AltitudeFt: 24000, Phase: "CRZ"},
	})
	id := storeTwo(t, txStore,
		"rehear les niveaux unite nine zero air france zero seven two",
		"Enchère : les niveaux, unité ninety, Air France zero six two")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	r := reread(t, txStore, id)
	if r.CallsignSource != "fr" {
		t.Fatalf("source: got %q, want fr", r.CallsignSource)
	}
	got := highlighted(r.ContentSecond, r.CallsignEvidence)
	if len(got) == 0 {
		t.Fatalf("no evidence stored for a match the second reading made")
	}
	for _, w := range got {
		if w == "<out of range>" {
			t.Fatalf("evidence %v does not fit the second reading %q", r.CallsignEvidence, r.ContentSecond)
		}
	}
	if want := []string{"Air France", "zero six two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("stored evidence highlights %q in %q, want %q", got, r.ContentSecond, want)
	}
}

// No match, no evidence: nothing to highlight is stored as nothing.
func TestNoMatchStoresNoEvidence(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, fixedFleet{
		{Callsign: "BAW572", Hex: "400a1b", AltitudeFt: 37000, Phase: "CRZ"},
	})
	id := store(t, txStore, "air france one zero eight one descend flight level one zero zero")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	if r := reread(t, txStore, id); r.Callsign != "" || r.CallsignEvidence != nil {
		t.Errorf("callsign %q with evidence %v, want neither", r.Callsign, r.CallsignEvidence)
	}
}

// movingFleet is a sky the test changes between two passes, as the receiver
// decodes an aircraft that has already spoken.
type movingFleet struct{ now []phraseology.Aircraft }

func (f *movingFleet) Fleet() []phraseology.Aircraft { return f.now }

// Heard on 26/09 on CDG approach: the first call came 17 s before the receiver
// decoded the aircraft at all, 39 s before its callsign. The transmission waits,
// and is attached on the pass where the aircraft appears -- values included.
func TestATransmissionWaitsForItsAircraftToBeDecoded(t *testing.T) {
	sky := &movingFleet{now: []phraseology.Aircraft{{Callsign: "DLH4AB", Hex: "3c4b26", AltitudeFt: 9000}}}
	proc, txStore, _ := newTestProcessor(t, sky)
	id := store(t, txStore, "lufthansa bien bonjour air algerie one two one four descending flight level one one zero")

	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	if r := reread(t, txStore, id); r.Callsign != "" {
		t.Fatalf("callsign %q before the aircraft was decoded", r.Callsign)
	}

	sky.now = append(sky.now, phraseology.Aircraft{Callsign: "DAH1214", Hex: "452134", AltitudeFt: 12150})
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	if r := reread(t, txStore, id); r.Callsign != "DAH1214" || len(r.CallsignEvidence) == 0 {
		t.Errorf("callsign %q, evidence %v: want DAH1214 with its words", r.Callsign, r.CallsignEvidence)
	}
	values, err := proc.valueStorage.ValuesByTranscription([]int64{id})
	if err != nil {
		t.Fatalf("ValuesByTranscription: %v", err)
	}
	if len(values[id]) == 0 {
		t.Fatal("the flight level was not stored")
	}
	for _, v := range values[id] {
		if v.Callsign != "DAH1214" {
			t.Errorf("value %s filed under %q, want DAH1214", v.Text, v.Callsign)
		}
	}
	if len(proc.pending) != 0 {
		t.Errorf("%d transmissions still waiting after the match", len(proc.pending))
	}
}

// Past retryFor the transmission is let go: an aircraft appearing later is a
// coincidence more often than the speaker.
func TestAWaitingTransmissionIsDroppedAfterAMinute(t *testing.T) {
	sky := &movingFleet{now: []phraseology.Aircraft{{Callsign: "BAW572", Hex: "400a1b", AltitudeFt: 37000}}}
	proc, txStore, _ := newTestProcessor(t, sky)
	clock := time.Date(2026, 9, 26, 14, 46, 0, 0, time.UTC)
	proc.now = func() time.Time { return clock }
	id := store(t, txStore, "air algerie one two one four descending flight level one one zero")

	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	clock = clock.Add(retryFor + time.Second)
	sky.now = append(sky.now, phraseology.Aircraft{Callsign: "DAH1214", Hex: "452134", AltitudeFt: 12150})
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	if r := reread(t, txStore, id); r.Callsign != "" {
		t.Errorf("callsign %q attached after the retry window", r.Callsign)
	}
	if len(proc.pending) != 0 {
		t.Errorf("%d transmissions still waiting past the window", len(proc.pending))
	}
}

// Without ADS-B there is no sky to wait for.
func TestNothingWaitsWithoutADSB(t *testing.T) {
	proc, txStore, _ := newTestProcessor(t, nil)
	store(t, txStore, "air algerie one two one four descending flight level one one zero")
	if err := proc.processNextBatch(); err != nil {
		t.Fatalf("processNextBatch: %v", err)
	}
	if len(proc.pending) != 0 {
		t.Errorf("%d transmissions waiting with no ADS-B", len(proc.pending))
	}
}
