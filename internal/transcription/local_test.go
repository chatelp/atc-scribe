package transcription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
	if v := got.Get("X-Created-At"); v != "2026-09-28T09:17:02+02:00" {
		t.Errorf("X-Created-At = %q, want created_at exactly as stored", v)
	}
	if v := got.Get("X-Segment-At"); v != "2026-09-28T07:17:02.345Z" {
		t.Errorf("X-Segment-At = %q, want the same moment in UTC to the millisecond", v)
	}
}
