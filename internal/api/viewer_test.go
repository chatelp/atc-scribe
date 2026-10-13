package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/config"
)

// A viewer account reads everything the page shows, and changes nothing; it
// is not given the audio. The owner, beside it, keeps everything.
func TestAViewerReadsAndChangesNothing(t *testing.T) {
	shared := newStack(t, stackOptions{viewer: true})
	id := shared.transcribe("mix", "AFR123 descend FL120", "AFR123", time.Now().Add(-time.Minute))

	status := func(s *stack) map[string]any {
		var out map[string]any
		rec := s.do("GET", "/api/v1/auth/status", "")
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	t.Run("reads", func(t *testing.T) {
		s := shared.with(t)
		s.signInAs(viewerUser)
		if st := status(s); st["role"] != "viewer" || st["user"] != viewerUser {
			t.Errorf("/auth/status for a viewer: %v", st)
		}
		for _, path := range []string{"/api/v1/aircraft", "/api/v1/aircraft/" + hexCruise, "/api/v1/frequencies",
			"/api/v1/transcriptions", "/api/v1/status", "/api/v1/config", "/api/v1/server", "/api/v1/station"} {
			if rec := s.do("GET", path, ""); rec.Code != http.StatusOK {
				t.Errorf("viewer GET %s: %d, want 200", path, rec.Code)
			}
		}
		conn := s.connectWS()
		conn.Close()
	})

	t.Run("is not given the audio", func(t *testing.T) {
		s := shared.with(t)
		s.signInAs(viewerUser)
		for _, req := range [][2]string{{"GET", "/api/v1/stream/x"}, {"HEAD", "/api/v1/stream/mix"},
			{"GET", "/api/v1/stream/mix?id=page-1"}, {"GET", "/api/v1/transcriptions/" + itoa(id) + "/audio"}} {
			if rec := s.do(req[0], req[1], ""); rec.Code != http.StatusForbidden {
				t.Errorf("viewer %s %s: %d, want 403", req[0], req[1], rec.Code)
			}
		}
	})

	t.Run("changes nothing", func(t *testing.T) {
		s := shared.with(t)
		s.signInAs(viewerUser)
		for _, req := range [][3]string{
			{"PUT", "/api/v1/server/settings", `{"db_retention_gb":1}`},
			{"PUT", "/api/v1/access", `{"mode":"local"}`},
			{"PUT", "/api/v1/sources/new", `{"url":"http://x/y"}`},
			{"DELETE", "/api/v1/sources/mix", ``},
			{"PUT", "/api/v1/frequencies/mix/label", `{"label":"x"}`},
			{"PUT", "/api/v1/receiver", `{"state":"chaine"}`},
		} {
			rec := s.do(req[0], req[1], req[2])
			if rec.Code != http.StatusForbidden {
				t.Errorf("viewer %s %s: %d, want 403", req[0], req[1], rec.Code)
			}
		}
		if got := shared.runtime.DBRetentionGB(); got == 1 {
			t.Error("the retention changed under a viewer")
		}
		// Signing out stays open to everyone.
		if rec := s.do("POST", "/api/v1/auth/logout", ""); rec.Code != http.StatusOK {
			t.Errorf("viewer signs out: %d", rec.Code)
		}
	})

	t.Run("the owner keeps everything", func(t *testing.T) {
		s := shared.with(t)
		s.signIn()
		if st := status(s); st["role"] != "owner" {
			t.Errorf("/auth/status for the owner: %v", st)
		}
		if rec := s.do("HEAD", "/api/v1/stream/mix", ""); rec.Code == http.StatusForbidden {
			t.Errorf("owner HEAD stream: %d", rec.Code)
		}
		if rec := s.do("GET", "/api/v1/transcriptions/"+itoa(id)+"/audio", ""); rec.Code == http.StatusForbidden {
			t.Errorf("owner clip: %d", rec.Code)
		}
		if rec := s.do("PUT", "/api/v1/server/settings", `{"db_retention_gb":12}`); rec.Code != http.StatusOK {
			t.Errorf("owner PUT /server/settings: %d %s", rec.Code, rec.Body.String())
		}
	})
}

// server.audio_replay = "all" gives viewers the audio too -- the stream and
// the clips -- and nothing else: they still change nothing.
func TestAudioReplayAllGivesViewersTheAudio(t *testing.T) {
	s := newStack(t, stackOptions{viewer: true})
	s.cfg.Server.AudioReplay = config.AudioReplayAll
	id := s.transcribe("mix", "AFR123 descend FL120", "AFR123", time.Now().Add(-time.Minute))
	s.signInAs(viewerUser)
	for _, req := range [][2]string{{"HEAD", "/api/v1/stream/mix"}, {"GET", "/api/v1/stream/x?id=page-1"},
		{"GET", "/api/v1/transcriptions/" + itoa(id) + "/audio"}} {
		if rec := s.do(req[0], req[1], ""); rec.Code == http.StatusForbidden {
			t.Errorf("viewer %s %s with audio_replay = all: 403", req[0], req[1])
		}
	}
	if rec := s.do("PUT", "/api/v1/server/settings", `{"db_retention_gb":1}`); rec.Code != http.StatusForbidden {
		t.Errorf("viewer PUT /server/settings with audio_replay = all: %d, want 403", rec.Code)
	}
}
