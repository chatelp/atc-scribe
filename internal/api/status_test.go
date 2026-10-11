package api

import (
	"net/http"
	"testing"
	"time"
)

// The receiver's state, as its reporter sends it, comes back in /status with
// its age; a report that stopped coming is said stale.
func TestTheReceiverStateIsReportedAndServed(t *testing.T) {
	s := newStack(t, stackOptions{})
	s.signIn()

	var st struct {
		Receiver *struct {
			State      string    `json:"state"`
			Label      string    `json:"label"`
			Title      string    `json:"title"`
			Since      string    `json:"since"`
			ReportedAt time.Time `json:"reported_at"`
			Stale      bool      `json:"stale"`
		} `json:"receiver"`
		ADSB *struct {
			Aircraft int `json:"aircraft"`
		} `json:"adsb"`
		AudioArchive struct {
			Available bool `json:"available"`
		} `json:"audio_archive"`
	}
	s.get("/api/v1/status", &st)
	if st.Receiver != nil || st.ADSB == nil {
		t.Fatalf("before any report: %+v", st)
	}

	report := `{"state":"lent","label":"Exploration SpyServer","title":"Selection libre - 124,350 + 126,425","since":"2026-10-11T09:00:00+02:00","mode":"exploration"}`
	if rec := s.do("PUT", "/api/v1/receiver", report); rec.Code != http.StatusNoContent {
		t.Fatalf("a report: %d %s", rec.Code, rec.Body.String())
	}
	s.get("/api/v1/status", &st)
	if st.Receiver == nil || st.Receiver.State != "lent" || st.Receiver.Label != "Exploration SpyServer" || st.Receiver.Stale {
		t.Fatalf("after a report: %+v", st.Receiver)
	}

	s.handler.status.mu.Lock()
	s.handler.status.reportedAt = time.Now().Add(-2 * time.Minute)
	s.handler.status.mu.Unlock()
	s.get("/api/v1/status", &st)
	if !st.Receiver.Stale {
		t.Error("a report two minutes old is stale")
	}

	for body, want := range map[string]int{
		`{"state":"broken"}`:           http.StatusBadRequest,
		`{"state":"on_air","extra":1}`: http.StatusBadRequest,
		`not json`:                     http.StatusBadRequest,
	} {
		if rec := s.do("PUT", "/api/v1/receiver", body); rec.Code != want {
			t.Errorf("PUT %s: %d, want %d", body, rec.Code, want)
		}
	}
	s.cookie = nil
	if rec := s.do("PUT", "/api/v1/receiver", `{"state":"off"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out: %d, want 401", rec.Code)
	}
}
