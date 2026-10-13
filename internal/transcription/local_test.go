package transcription

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/websocket"
)

// An archived clip must join its database row exactly. The sidecar's own clock
// cannot do it -- under a queue it sees a transmission up to a minute after it
// ended -- so the transmission's date travels with the audio, in the very form
// created_at is stored in.
func TestTheSidecarIsToldTheTransmissionsOwnDate(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		// A rejection stores nothing, so the test needs no database.
		_, _ = w.Write([]byte(`{"text":"","rejected":"no_speech"}`))
	}))
	t.Cleanup(srv.Close)

	p := &LocalProcessor{
		frequencyID: "124350",
		language:    "en",
		config:      Config{Local: LocalSTTConfig{ServerURL: srv.URL}},
		client:      srv.Client(),
		ctx:         context.Background(),
		logger:      testLogger(t),
	}
	paris := time.FixedZone("CEST", 2*3600)
	at := time.Date(2026, 9, 28, 9, 17, 2, 345_000_000, paris)
	p.send([]byte{0, 0, 0, 0}, 24000, at)

	if got == nil {
		t.Fatal("the sidecar was never called")
	}
	if v := got.Get("X-Created-At"); v != "2026-09-28T07:17:02Z" {
		t.Errorf("X-Created-At = %q, want created_at exactly as stored (UTC since 10/10)", v)
	}
	if v := got.Get("X-Segment-At"); v != "2026-09-28T07:17:02.345Z" {
		t.Errorf("X-Segment-At = %q, want the same moment in UTC to the millisecond", v)
	}
}

// The rolling mean: the first transmission sets it, each next one moves it a
// tenth of the way; nothing is said before the first.
func TestTheTimePerTransmissionIsARollingMean(t *testing.T) {
	var timing TranscriptionTiming
	if _, ok := timing.Mean(); ok {
		t.Fatal("a mean before any transmission")
	}
	timing.Record(2 * time.Second)
	if m, ok := timing.Mean(); !ok || m != 2 {
		t.Fatalf("after one transmission of 2 s: %v %v, want 2", m, ok)
	}
	timing.Record(4 * time.Second)
	if m, _ := timing.Mean(); math.Abs(m-2.2) > 1e-9 {
		t.Errorf("after 2 s then 4 s: %v, want 2.2", m)
	}
	for i := 0; i < 100; i++ {
		timing.Record(time.Second)
	}
	if m, _ := timing.Mean(); math.Abs(m-1) > 1e-3 {
		t.Errorf("after a hundred transmissions of 1 s: %v, want close to 1", m)
	}
	var none *TranscriptionTiming
	none.Record(time.Second) // a processor built without one records nothing
	if _, ok := none.Mean(); ok {
		t.Error("a nil timing has a mean")
	}
}

// Only a transmission that was transcribed counts: a rejection costs a
// fraction of a decode and would pull the figure down.
func TestOnlyTranscribedTransmissionsAreTimed(t *testing.T) {
	answer := `{"text":"","rejected":"no_speech"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)

	log := testLogger(t)
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"), log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ws := websocket.NewServer(log)
	go ws.Run()

	timing := &TranscriptionTiming{}
	p := &LocalProcessor{
		frequencyID: "124350",
		language:    "en",
		config:      Config{Local: LocalSTTConfig{ServerURL: srv.URL}},
		client:      srv.Client(),
		timing:      timing,
		ctx:         context.Background(),
		logger:      log,
		sink: &transcriptionSink{frequencyID: "124350", storage: sqlite.NewTranscriptionStorage(db, log),
			wsServer: ws, logger: log},
	}
	p.send([]byte{0, 0, 0, 0}, 16000, time.Now())
	if _, ok := p.MeanTranscriptionSeconds(); ok {
		t.Fatal("a rejection was timed")
	}
	answer = `{"text":"air france one two three","language":"en"}`
	p.send([]byte{0, 0, 0, 0}, 16000, time.Now())
	if m, ok := p.MeanTranscriptionSeconds(); !ok || m < 0.02 || m > 5 {
		t.Errorf("after one transcription of about 20 ms: %v %v", m, ok)
	}
}
