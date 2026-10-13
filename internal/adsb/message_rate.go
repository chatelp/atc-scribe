package adsb

import (
	"sync"
	"time"
)

// messageRate derives messages per second from the receiver's running
// "messages" counter (aircraft.json), sampled once per fetch. A health figure
// for the status bar (13/10): an antenna that hears 300 messages a second is
// working, one that hears 3 is not, whatever the aircraft count says. The
// window is short, ten samples, so a restart of the receiver (the counter
// drops) only blanks the figure for a few seconds.
type messageRate struct {
	mu      sync.Mutex
	samples []messageSample
}

type messageSample struct {
	at       time.Time
	messages int
}

const messageRateWindow = 10

// note records the counter as read now. A source that reports no counter
// (external APIs give 0) leaves the rate unknown.
func (r *messageRate) note(messages int, at time.Time) {
	if messages <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.samples); n > 0 && messages < r.samples[n-1].messages {
		// The receiver restarted: the old samples would give a negative rate.
		r.samples = r.samples[:0]
	}
	r.samples = append(r.samples, messageSample{at: at, messages: messages})
	if len(r.samples) > messageRateWindow {
		r.samples = r.samples[len(r.samples)-messageRateWindow:]
	}
}

// perSecond is the rate over the kept samples, 0 while it is unknown (fewer
// than two samples, or no time between them).
func (r *messageRate) perSecond() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.samples)
	if n < 2 {
		return 0
	}
	first, last := r.samples[0], r.samples[n-1]
	secs := last.at.Sub(first.at).Seconds()
	if secs <= 0 {
		return 0
	}
	return float64(last.messages-first.messages) / secs
}
