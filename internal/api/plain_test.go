package api

import (
	"fmt"
	"testing"
	"time"
)

// Every transmission served carries what it says in plain words, read from its
// text by the phraseology grammar.
func TestTransmissionsAreServedInPlainWords(t *testing.T) {
	s := newStack(t, stackOptions{})
	s.signIn()
	at := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	id := s.transcribe("124350", "air france one two three descend flight level one zero zero contact one one nine decimal two five five", "", at)
	s.transcribe("124350", "bonjour", "", at.Add(time.Second))

	var page struct {
		Transcriptions []struct {
			ID    int64 `json:"id"`
			Plain []struct {
				Role  string `json:"role"`
				Text  string `json:"text"`
				Value string `json:"value"`
				Unit  string `json:"unit"`
			} `json:"plain"`
		} `json:"transcriptions"`
	}
	for _, path := range []string{"/api/v1/transcriptions", "/api/v1/transcriptions/frequency/124350"} {
		s.get(path, &page)
		found := false
		for _, tr := range page.Transcriptions {
			if tr.ID != id {
				if len(tr.Plain) != 0 {
					t.Errorf("%s: %d says %v, want nothing", path, tr.ID, tr.Plain)
				}
				continue
			}
			found = true
			if got := fmt.Sprint(tr.Plain); got != "[{flight_level Descend to FL100 100 FL} {frequency Contact 119.255 119.255 MHz}]" {
				t.Errorf("%s: %s", path, got)
			}
		}
		if !found {
			t.Errorf("%s: transmission %d missing", path, id)
		}
	}
}
