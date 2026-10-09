package sqlite

import (
	"testing"
	"time"
)

func newTranscriptionStore(t *testing.T) *TranscriptionStorage {
	t.Helper()
	return NewTranscriptionStorage(newAircraftStore(t).GetDB(), testLog(t))
}

func store(t *testing.T, s *TranscriptionStorage, r TranscriptionRecord) int64 {
	t.Helper()
	id, err := s.StoreTranscription(&r)
	if err != nil {
		t.Fatalf("StoreTranscription: %v", err)
	}
	return id
}

func heard(freq string, at time.Time, content string) TranscriptionRecord {
	return TranscriptionRecord{FrequencyID: freq, CreatedAt: at, Content: content, IsComplete: true}
}

// Every column written by StoreTranscription is read back, and an unset
// optional column reads as empty, not as an error.
func TestATranscriptionIsReadBackWhole(t *testing.T) {
	s := newTranscriptionStore(t)
	full := TranscriptionRecord{
		FrequencyID: "125825", CreatedAt: t0, Content: "air france one zero eight one",
		IsComplete: true, IsProcessed: true, ContentProcessed: "AFR1081",
		SpeakerType: "PILOT", Callsign: "AFR1081", Language: "fr",
		ContentSecond: "air france un zéro huit un", CallsignSource: "second",
	}
	id := store(t, s, full)
	store(t, s, heard("125825", t0.Add(-time.Minute), "bare"))

	got, err := s.GetTranscriptions(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d records, want 2", len(got))
	}
	r := got[0]
	if r.ID != id || !r.CreatedAt.Equal(t0) || !r.IsComplete || !r.IsProcessed {
		t.Errorf("id/time/flags: %+v", r)
	}
	for field, pair := range map[string][2]string{
		"frequency_id":      {r.FrequencyID, full.FrequencyID},
		"content":           {r.Content, full.Content},
		"content_processed": {r.ContentProcessed, full.ContentProcessed},
		"speaker_type":      {r.SpeakerType, full.SpeakerType},
		"callsign":          {r.Callsign, full.Callsign},
		"language":          {r.Language, full.Language},
		"content_second":    {r.ContentSecond, full.ContentSecond},
		"callsign_source":   {r.CallsignSource, full.CallsignSource},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s: got %q, want %q", field, pair[0], pair[1])
		}
	}
	bare := got[1]
	if bare.Callsign != "" || bare.ContentSecond != "" || bare.CallsignEvidence != nil || bare.IsProcessed {
		t.Errorf("unset columns should read empty: %+v", bare)
	}
}

// The post-processor's queue: complete and unprocessed, oldest first; once
// processed a transmission leaves it, with its callsign and source, and with no
// evidence stored when there is none.
func TestTheUnprocessedQueueIsOldestFirstAndEmptiesAsItIsProcessed(t *testing.T) {
	s := newTranscriptionStore(t)
	late := store(t, s, heard("125825", t0.Add(time.Minute), "later"))
	early := store(t, s, heard("125825", t0, "earlier"))
	partial := heard("125825", t0.Add(-time.Minute), "partial")
	partial.IsComplete = false
	store(t, s, partial)

	queue, err := s.GetUnprocessedTranscriptions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 2 || queue[0].ID != early || queue[1].ID != late {
		t.Fatalf("queue %v, want [%d %d]", ids(queue), early, late)
	}

	if err := s.UpdateProcessedTranscription(early, "earlier, processed", "ATC", "AFR1081", "primary"); err != nil {
		t.Fatal(err)
	}
	queue, _ = s.GetUnprocessedTranscriptions(10)
	if len(queue) != 1 || queue[0].ID != late {
		t.Fatalf("after processing one: %v, want [%d]", ids(queue), late)
	}

	var evidence *string
	if err := s.db.QueryRow(`SELECT callsign_evidence FROM transcriptions WHERE id = ?`, early).Scan(&evidence); err != nil {
		t.Fatal(err)
	}
	if evidence != nil {
		t.Errorf("no evidence should be stored as NULL, got %q", *evidence)
	}
	done, err := s.GetLastProcessedTranscriptions("125825", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 || done[0].Callsign != "AFR1081" || done[0].CallsignSource != "primary" ||
		done[0].SpeakerType != "ATC" || done[0].ContentProcessed != "earlier, processed" {
		t.Errorf("processed transmission read back as %+v", done)
	}
}

func ids(rs []*TranscriptionRecord) []int64 {
	out := make([]int64, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

// The history pages: per frequency or per speaker, newest first, with limit
// and offset; a time range is inclusive at both ends.
func TestHistoryPagesFilterAndPaginate(t *testing.T) {
	s := newTranscriptionStore(t)
	var onFreq []int64
	for i := 0; i < 5; i++ {
		r := heard("125825", t0.Add(time.Duration(i)*time.Minute), "on 125.825")
		if i%2 == 0 {
			r.SpeakerType = "ATC"
		}
		onFreq = append(onFreq, store(t, s, r))
	}
	store(t, s, heard("120850", t0.Add(10*time.Minute), "elsewhere"))

	page, err := s.GetTranscriptionsByFrequency("125825", 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(page); len(got) != 2 || got[0] != onFreq[3] || got[1] != onFreq[2] {
		t.Errorf("second and third newest on 125825: %v, want [%d %d]", got, onFreq[3], onFreq[2])
	}

	atc, err := s.GetTranscriptionsBySpeaker("ATC", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(atc); len(got) != 3 || got[0] != onFreq[4] || got[2] != onFreq[0] {
		t.Errorf("ATC newest first: %v", got)
	}

	window, err := s.GetTranscriptionsByTimeRange(t0.Add(time.Minute), t0.Add(3*time.Minute), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(window); len(got) != 3 || got[0] != onFreq[3] || got[2] != onFreq[1] {
		t.Errorf("minutes 1 to 3 inclusive: %v, want [%d %d %d]", got, onFreq[3], onFreq[2], onFreq[1])
	}
}

// What the map shows for an aircraft heard on the radio: how many times, when
// last, and the last words -- ignoring the transmissions that named nobody.
func TestVoiceSummariesCountAndKeepTheLastWords(t *testing.T) {
	s := newTranscriptionStore(t)
	for i, cs := range []string{"AFR1081", "AFR1081", "", "  ", "EZY45AB"} {
		r := heard("125825", t0.Add(time.Duration(i)*time.Minute), "words "+string(rune('a'+i)))
		r.Callsign = cs
		store(t, s, r)
	}

	got, err := s.VoiceSummaries()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("two callsigns heard, got %v", got)
	}
	afr := got["AFR1081"]
	if afr.Transmissions != 2 || afr.LastText != "words b" || !afr.LastHeard.Equal(t0.Add(time.Minute)) {
		t.Errorf("AFR1081: %+v, want 2 transmissions, last at %v saying %q", afr, t0.Add(time.Minute), "words b")
	}
	if ezy := got["EZY45AB"]; ezy.Transmissions != 1 || ezy.LastText != "words e" {
		t.Errorf("EZY45AB: %+v", ezy)
	}
}

// The values the grammar extracts are stored per transmission, read back for a
// page of transmissions in one query, refiled under an aircraft decoded after
// it spoke, and a processed transmission without values is found again.
func TestPhraseologyValuesFollowTheirTransmission(t *testing.T) {
	s := newTranscriptionStore(t)
	v := NewPhraseologyStorage(DBOf(s))
	processed := func(at time.Time) int64 {
		r := heard("125825", at, "climb flight level three five zero")
		r.IsProcessed = true
		return store(t, s, r)
	}
	a, b, c := processed(t0), processed(t0.Add(time.Minute)), processed(t0.Add(2*time.Minute))
	store(t, s, heard("125825", t0.Add(3*time.Minute), "not processed yet"))

	if err := v.StoreValues([]PhraseologyValue{
		{TranscriptionID: a, Role: "flight_level", Digits: "350", Text: "FL350", CreatedAt: t0},
		{TranscriptionID: a, Role: "heading", Digits: "270", Text: "270", CreatedAt: t0},
		{TranscriptionID: b, Callsign: "AFR1081", Role: "squawk", Digits: "4521", Text: "4521", CreatedAt: t0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := v.StoreValues(nil); err != nil {
		t.Errorf("nothing to store is not an error: %v", err)
	}

	byTx, err := v.ValuesByTranscription([]int64{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTx[a]) != 2 || byTx[a][0].Text != "FL350" || byTx[a][1].Role != "heading" || len(byTx[b]) != 1 || len(byTx[c]) != 0 {
		t.Fatalf("values by transmission: %+v", byTx)
	}
	if !byTx[a][0].CreatedAt.Equal(t0) {
		t.Errorf("created_at read back as %v, want %v", byTx[a][0].CreatedAt, t0)
	}

	if err := v.SetCallsign(a, "EZY45AB"); err != nil {
		t.Fatal(err)
	}
	byTx, _ = v.ValuesByTranscription([]int64{a})
	for _, val := range byTx[a] {
		if val.Callsign != "EZY45AB" {
			t.Errorf("value %s not refiled: %q", val.Role, val.Callsign)
		}
	}

	pending, err := v.TranscriptionsWithoutValues(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != c || !pending[0].CreatedAt.Equal(t0.Add(2*time.Minute)) {
		t.Errorf("processed without values: %+v, want only %d", pending, c)
	}
	if empty, err := v.ValuesByTranscription(nil); err != nil || len(empty) != 0 {
		t.Errorf("no ids: %v, %v", empty, err)
	}
}
