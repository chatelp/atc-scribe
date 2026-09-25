package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Trimmed from the station's /radio/etat and /radio/frequences, 25/09/2026.
const stateJSON = `{"airband": "running", "version": 19, "depuis": "2026-09-25T21:19:26+02:00",
 "selection": {"nom": "en-route-et-descente-cdg", "titre": "En route et descente CDG - 132-133 MHz",
  "frequences": [
   {"designation": "132.275", "mhz": 132.275, "id": "132275", "service": "Paris Controle - secteur arrivee CDG",
    "langue": "mixte", "flux": "/aero-132275.mp3"},
   {"designation": "132.733", "mhz": 132.733333, "id": "132733", "service": "en route - croisiere haute",
    "langue": "en", "flux": "/aero-132733.mp3"},
   {"designation": "118.000", "mhz": 118.0, "id": "118000", "service": "Saint-Cyr Tour (LFPZ)",
    "langue": "fr", "flux": "/aero-118000.mp3"},
   {"designation": "121.500", "mhz": 121.5, "id": "121500", "service": "no stream yet", "langue": "en"}]}}`

const catalogueJSON = `{"frequences": [{"id": "132275", "transcrire": true}, {"id": "132733", "transcrire": false},
 {"id": "118000", "transcrire": true}]}`

func parsed(t *testing.T) (stationState, catalogue) {
	t.Helper()
	var st stationState
	var cat catalogue
	if err := json.Unmarshal([]byte(stateJSON), &st); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(catalogueJSON), &cat); err != nil {
		t.Fatal(err)
	}
	return st, cat
}

func TestEachChannelWithAStreamBecomesASource(t *testing.T) {
	st, cat := parsed(t)
	w := wanted(st, cat, "http://audio.lan/")
	if len(w) != 3 {
		t.Fatalf("3 channels have a stream, got %d sources: %v", len(w), w)
	}
	got := w["132275"]
	want := source{Name: "132.275 Paris Controle - secteur arrivee CDG", FrequencyMHz: 132.275,
		URL: "http://audio.lan/aero-132275.mp3", Order: 132275, TranscribeAudio: true, Language: "en"}
	if got != want {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if w["118000"].Language != "fr" {
		t.Errorf("a French channel is read in French: %+v", w["118000"])
	}
	if w["132733"].TranscribeAudio {
		t.Error("the catalogue said not to transcribe 132733")
	}
	if w["132275"].FFmpegReconnect {
		t.Error("a channel's stream can disappear: ffmpeg must not loop on it")
	}
}

// With the receiver off or lent to another program there are no streams, and
// co-atc should stop reading them rather than retry them.
func TestNoSourcesWhenTheReceiverIsNotRunning(t *testing.T) {
	st, cat := parsed(t)
	st.Airband = "exited"
	if w := wanted(st, cat, "http://audio.lan"); len(w) != 0 {
		t.Errorf("got %v, want nothing", w)
	}
}

func TestThePlanOnlyTouchesWhatChanged(t *testing.T) {
	st, cat := parsed(t)
	want := wanted(st, cat, "http://audio.lan")
	sent := map[string]source{}
	var have []listed
	for id, s := range want {
		sent[id] = s
		have = append(have, listed{ID: id, Name: s.Name, URL: s.URL, FrequencyMHz: s.FrequencyMHz,
			Order: s.Order, TranscribeAudio: s.TranscribeAudio, Runtime: true})
	}
	// the configured mixed stream is not ours to touch
	have = append(have, listed{ID: "aero-melange", URL: "http://audio.lan/aero.mp3"})
	// a channel the station no longer listens to
	have = append(have, listed{ID: "124350", URL: "http://audio.lan/aero-124350.mp3", Runtime: true})

	set, remove := plan(want, have, sent)
	if len(set) != 0 {
		t.Errorf("nothing changed, nothing to set: %v", set)
	}
	if !reflect.DeepEqual(remove, []string{"124350"}) {
		t.Errorf("remove = %v, want only the channel that left", remove)
	}

	// co-atc restarted: it has none of them any more
	set, _ = plan(want, have[len(have)-2:len(have)-1], sent)
	if len(set) != 3 {
		t.Errorf("after a co-atc restart all 3 are set again, got %v", set)
	}
}
