package adsb

import (
	"errors"
	"testing"
	"time"
)

func TestTheMessageRateFollowsTheReceiversCounter(t *testing.T) {
	var r messageRate
	t0 := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

	if got := r.perSecond(); got != 0 {
		t.Fatalf("rate before any sample = %v, want 0", got)
	}
	r.note(1000, t0)
	if got := r.perSecond(); got != 0 {
		t.Fatalf("rate after one sample = %v, want 0 (unknown)", got)
	}
	for i := 1; i <= 5; i++ {
		r.note(1000+i*250, t0.Add(time.Duration(i)*time.Second))
	}
	if got := r.perSecond(); got != 250 {
		t.Fatalf("rate over five seconds at 250/s = %v, want 250", got)
	}

	// The window keeps the last ten samples only.
	for i := 6; i <= 30; i++ {
		r.note(1000+i*250, t0.Add(time.Duration(i)*time.Second))
	}
	if n := len(r.samples); n != messageRateWindow {
		t.Fatalf("kept %d samples, want %d", n, messageRateWindow)
	}

	// The receiver restarts: the counter drops, the rate is unknown again
	// until two new samples, never negative.
	r.note(40, t0.Add(31*time.Second))
	if got := r.perSecond(); got != 0 {
		t.Fatalf("rate right after a counter reset = %v, want 0", got)
	}
	r.note(340, t0.Add(32*time.Second))
	if got := r.perSecond(); got != 300 {
		t.Fatalf("rate after the reset = %v, want 300", got)
	}

	// A source without a counter leaves everything as it was.
	before := r.perSecond()
	r.note(0, t0.Add(33*time.Second))
	if got := r.perSecond(); got != before {
		t.Fatalf("a zero counter changed the rate: %v -> %v", before, got)
	}
}

// A failed fetch is said in a few words a reader can act on (14/10).
func TestFetchFailureReason(t *testing.T) {
	cases := map[string]string{
		`Get "http://station:8080/data/aircraft.json": dial tcp 10.0.0.1:8080: connect: no route to host`: "no route to the station",
		`dial tcp 10.0.0.1:8080: i/o timeout`:                 "the station does not answer in time",
		`dial tcp 10.0.0.1:8080: connect: connection refused`: "the station refuses the connection",
		`unexpected status 502`:                               "the station answers with an error",
		`something else`:                                      "the station cannot be read",
	}
	for msg, want := range cases {
		if got := fetchFailureReason(errors.New(msg)); got != want {
			t.Errorf("%q: %q, want %q", msg, got, want)
		}
	}
}
