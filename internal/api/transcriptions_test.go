package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/storage/sqlite"
)

type transcriptionsPage struct {
	Count          int                                  `json:"count"`
	FrequencyID    string                               `json:"frequency_id"`
	Callsign       string                               `json:"callsign"`
	Transcriptions []sqlite.TranscriptionRecord         `json:"transcriptions"`
	Values         map[string][]sqlite.PhraseologyValue `json:"values"`
}

func (p transcriptionsPage) contents() []string {
	out := make([]string, 0, len(p.Transcriptions))
	for _, t := range p.Transcriptions {
		out = append(out, t.Content)
	}
	return out
}

var (
	t0 = time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Minute)
	t2 = t0.Add(2 * time.Minute)
)

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// The transcription routes, all read-only, on one stack holding three
// transmissions as the transcription side leaves them: one nobody was found
// for, one from RYR45 on the mix, one from AFR123 on the other frequency
// with what the grammar took from it.
func TestTranscriptionRoutes(t *testing.T) {
	shared := newStack(t, stackOptions{})
	shared.signIn()
	unmatched := shared.transcribe("mix", "orly tower bonjour", "", t0)
	if err := shared.tx.UpdateMatchedTranscription(unmatched, "Orly tower bonjour", "ATC", "", "", nil); err != nil {
		t.Fatal(err)
	}
	shared.transcribe("mix", "ryanair four five descend", "RYR45", t1)
	afr := shared.transcribe("other", "air france one two three climb flight level three five zero", "AFR123", t2)
	if err := shared.values.StoreValues([]sqlite.PhraseologyValue{
		{TranscriptionID: afr, Callsign: "AFR123", Role: "flight_level", Digits: "350", Text: "FL350", CreatedAt: t2},
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("listed newest first with their fields", func(t *testing.T) {
		s := shared.with(t)
		var page transcriptionsPage
		s.get("/api/v1/transcriptions", &page)
		if page.Count != 3 || len(page.Transcriptions) != 3 {
			t.Fatalf("count %d with %d transcriptions, want 3", page.Count, len(page.Transcriptions))
		}
		if got := page.contents(); got[0] != "air france one two three climb flight level three five zero" || got[2] != "orly tower bonjour" {
			t.Errorf("order: %v, want newest first", got)
		}
		a := page.Transcriptions[0]
		if a.ID != afr || a.FrequencyID != "other" || !a.CreatedAt.Equal(t2) || !a.IsComplete || !a.IsProcessed ||
			a.Language != "en" || a.SpeakerType != "PILOT" || a.Callsign != "AFR123" {
			t.Errorf("AFR123's transmission = %+v", a)
		}
		// The three fields the page reads as they are: which reading the
		// callsign came from, where the words are in it, and that reading.
		if a.CallsignSource != "second" || len(a.CallsignEvidence) != 1 || a.CallsignEvidence[0] != [2]int{0, 6} ||
			!strings.HasPrefix(a.ContentSecond, "deuxième lecture") {
			t.Errorf("callsign_source %q, callsign_evidence %v, content_second %q", a.CallsignSource, a.CallsignEvidence, a.ContentSecond)
		}
		// Stored in UTC and answered as such.
		rec := s.do("GET", "/api/v1/transcriptions?limit=1", "")
		if body := rec.Body.String(); !strings.Contains(body, `"created_at":"2026-10-10T08:02:00Z"`) {
			t.Errorf("created_at is not the UTC instant stored: %s", body)
		}
		u := page.Transcriptions[2]
		if u.SpeakerType != "ATC" || u.Callsign != "" || u.CallsignSource != "" || u.CallsignEvidence != nil {
			t.Errorf("the unmatched transmission = %+v, want ATC and no callsign fields", u)
		}

		s.get("/api/v1/transcriptions?limit=2&offset=1", &page)
		if got := page.contents(); page.Count != 2 || got[0] != "ryanair four five descend" || got[1] != "orly tower bonjour" {
			t.Errorf("limit=2&offset=1: %v", got)
		}
		s.get("/api/v1/transcriptions?limit=0&offset=-3", &page)
		if page.Count != 3 {
			t.Errorf("unusable paging falls back to the defaults: got %d", page.Count)
		}
	})

	t.Run("by frequency", func(t *testing.T) {
		s := shared.with(t)
		var page transcriptionsPage
		s.get("/api/v1/transcriptions/frequency/mix", &page)
		if page.FrequencyID != "mix" || page.Count != 2 || page.Transcriptions[0].Callsign != "RYR45" {
			t.Errorf("mix: %+v", page)
		}
		s.get("/api/v1/transcriptions/frequency/other?limit=1", &page)
		if page.Count != 1 || page.Transcriptions[0].Callsign != "AFR123" {
			t.Errorf("other: %+v", page)
		}
		// A frequency nobody spoke on answers count 0 and "transcriptions":
		// null (a nil slice); the page tests the field before mapping it, so
		// null is what it expects.
		rec := s.do("GET", "/api/v1/transcriptions/frequency/silent", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"count":0`) ||
			!strings.Contains(rec.Body.String(), `"transcriptions":null`) {
			t.Errorf("a frequency nobody spoke on: %d %s", rec.Code, rec.Body.String())
		}
	})

	// What the page asks when an aircraft is selected: its transmissions and
	// what the grammar took from them, keyed by transmission.
	t.Run("by callsign, with the values", func(t *testing.T) {
		s := shared.with(t)
		var page transcriptionsPage
		s.get("/api/v1/transcriptions/callsign/AFR123?limit=50", &page)
		if page.Callsign != "AFR123" || page.Count != 1 || page.Transcriptions[0].ID != afr {
			t.Fatalf("AFR123: %+v", page)
		}
		vals := page.Values[itoa(afr)]
		if len(vals) != 1 || vals[0].Role != "flight_level" || vals[0].Text != "FL350" || vals[0].Digits != "350" ||
			vals[0].TranscriptionID != afr {
			t.Errorf("values for AFR123's transmission = %+v, want FL350", vals)
		}
		var ryr transcriptionsPage // fresh: decoding into a map keeps its old keys
		s.get("/api/v1/transcriptions/callsign/RYR45", &ryr)
		if ryr.Count != 1 || len(ryr.Values) != 0 {
			t.Errorf("RYR45: count %d, values %v, want one transmission and no values", ryr.Count, ryr.Values)
		}
		// Exact callsign, as the page sends it from the aircraft's flight.
		var prefix transcriptionsPage
		s.get("/api/v1/transcriptions/callsign/AFR", &prefix)
		if prefix.Count != 0 || prefix.Values == nil {
			t.Errorf("a prefix: count %d, values %v (want nothing, and an empty object)", prefix.Count, prefix.Values)
		}
	})

	t.Run("by speaker and by time range", func(t *testing.T) {
		s := shared.with(t)
		var page transcriptionsPage
		s.get("/api/v1/transcriptions/speaker/ATC", &page)
		if page.Count != 1 || page.Transcriptions[0].Content != "orly tower bonjour" {
			t.Errorf("ATC: %v", page.contents())
		}
		s.get("/api/v1/transcriptions/speaker/PILOT", &page)
		if page.Count != 2 {
			t.Errorf("PILOT: %v", page.contents())
		}
		if rec := s.do("GET", "/api/v1/transcriptions/speaker/pilot", ""); rec.Code != http.StatusBadRequest {
			t.Errorf("a speaker type in lower case: %d, want 400 (ATC or PILOT)", rec.Code)
		}

		if rec := s.do("GET", "/api/v1/transcriptions/time-range", ""); rec.Code != http.StatusBadRequest ||
			!strings.Contains(rec.Body.String(), "start_time") {
			t.Errorf("time-range without start_time: %d %s", rec.Code, rec.Body.String())
		}
		if rec := s.do("GET", "/api/v1/transcriptions/time-range?start_time=yesterday", ""); rec.Code != http.StatusBadRequest {
			t.Errorf("an unparseable start_time: %d, want 400", rec.Code)
		}
		if rec := s.do("GET", "/api/v1/transcriptions/time-range?start_time=2026-10-10T08:00:00Z&end_time=noon", ""); rec.Code != http.StatusBadRequest {
			t.Errorf("an unparseable end_time: %d, want 400", rec.Code)
		}
		s.get("/api/v1/transcriptions/time-range?start_time=2026-10-10T08:00:30Z&end_time=2026-10-10T08:01:30Z", &page)
		if got := page.contents(); len(got) != 1 || got[0] != "ryanair four five descend" {
			t.Errorf("08:00:30 to 08:01:30: %v, want RYR45's alone", got)
		}
		// The same bounds in another zone name the same instants.
		s.get("/api/v1/transcriptions/time-range?start_time=2026-10-10T10:00:30%2B02:00&end_time=2026-10-10T10:01:30%2B02:00", &page)
		if got := page.contents(); len(got) != 1 || got[0] != "ryanair four five descend" {
			t.Errorf("the same range given in +02:00: %v, want RYR45's alone", got)
		}
		// No end: until now.
		s.get("/api/v1/transcriptions/time-range?start_time=2026-10-10T08:01:30Z", &page)
		if got := page.contents(); len(got) != 1 || got[0] != "air france one two three climb flight level three five zero" {
			t.Errorf("from 08:01:30 on: %v", got)
		}
	})
}
