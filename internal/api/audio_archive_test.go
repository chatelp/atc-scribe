package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/audioarchive"
)

// A transmission's clip, as the sidecar archived it, is served back by its
// transmission's ID; only signed in, and only from inside the archive.
func TestATransmissionCanBeListenedToAgain(t *testing.T) {
	s := newStack(t, stackOptions{})
	s.signIn()

	var state map[string]bool
	s.get("/api/v1/audio-archive", &state)
	if state["configured"] || state["available"] {
		t.Fatalf("without an archive: %v", state)
	}

	root := t.TempDir()
	s.handler.audioArchive = audioarchive.New(root)
	at := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	day := at.Local().Format("20060102")
	if err := os.MkdirAll(filepath.Join(root, day, "124350"), 0o755); err != nil {
		t.Fatal(err)
	}
	clip := []byte("RIFF0000WAVEfmt fake audio")
	if err := os.WriteFile(filepath.Join(root, day, "124350", "t.wav"), clip, 0o644); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"frequency_id": "124350", "created_at": %q, "fichier": "%s/124350/t.wav"}`+"\n", at.Format(time.RFC3339), day)
	if err := os.WriteFile(filepath.Join(root, day, "manifeste.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	kept := s.transcribe("124350", "air france one two three descend", "", at)
	notKept := s.transcribe("124350", "roger", "", at.Add(5*time.Second))

	s.get("/api/v1/audio-archive", &state)
	if !state["configured"] || !state["available"] {
		t.Fatalf("with an archive: %v", state)
	}

	rec := s.do("GET", fmt.Sprintf("/api/v1/transcriptions/%d/audio", kept), "")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "audio/wav" || rec.Body.String() != string(clip) {
		t.Fatalf("the clip: %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	// The browser seeks with ranges.
	rec = s.do("GET", fmt.Sprintf("/api/v1/transcriptions/%d/audio", kept), "", func(r *http.Request) {
		r.Header.Set("Range", "bytes=0-3")
	})
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "RIFF" {
		t.Errorf("a range: %d %q", rec.Code, rec.Body.String())
	}
	for path, want := range map[string]int{
		fmt.Sprintf("/api/v1/transcriptions/%d/audio", notKept): http.StatusNotFound,
		"/api/v1/transcriptions/999999/audio":                   http.StatusNotFound,
		"/api/v1/transcriptions/abc/audio":                      http.StatusBadRequest,
	} {
		if rec := s.do("GET", path, ""); rec.Code != want {
			t.Errorf("GET %s: %d, want %d", path, rec.Code, want)
		}
	}

	s.cookie = nil
	if rec := s.do("GET", fmt.Sprintf("/api/v1/transcriptions/%d/audio", kept), ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out: %d, want 401", rec.Code)
	}

	// An unplugged disk: the archive is configured but not there.
	s.signIn()
	s.handler.audioArchive = audioarchive.New(filepath.Join(root, "unplugged"))
	s.get("/api/v1/audio-archive", &state)
	if !state["configured"] || state["available"] {
		t.Errorf("unplugged: %v", state)
	}
	if rec := s.do("GET", fmt.Sprintf("/api/v1/transcriptions/%d/audio", kept), ""); rec.Code != http.StatusNotFound {
		t.Errorf("unplugged: %d, want 404", rec.Code)
	}
}
