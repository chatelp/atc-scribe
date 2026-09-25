package transcription

import "time"

// streamClock dates audio by where it sits in the stream, not by when it was read.
//
// An Icecast server greets each new listener with a burst of audio it has
// already sent to others -- 64 KB by default, which is 50 to 65 seconds of an
// 8 kHz variable-bitrate MP3 stream -- and the listener receives it within a
// second. Stamping transmissions by arrival dates a minute of them to the moment
// of connection. Past the burst, audio arrives about as it is produced, and
// arrival time is right again.
//
// The clock keeps how far the audio received so far runs ahead of the wall
// clock since the first sample: the lead. Once the burst is in, the lead stops
// growing, and its largest recent value is the burst plus the fastest delivery
// seen. Audio that arrived with a smaller lead arrived late by the difference,
// and is dated that much earlier than its arrival.
//
// While the lead is still growing faster than the clock -- a connection's burst,
// or a reconnection's -- the final lead is not known yet, and neither is the date
// of what is being received. Transmissions cut in that time wait for it (see
// mark and settled). A reconnection shows as a gap in delivery, after which the
// old connection's leads are forgotten; a stall without one is only lateness.
type streamClock struct {
	rate    float64
	started bool
	start   time.Time
	samples int64

	// recent leads, oldest first, for telling a burst from steady delivery
	recent []leadAt
	// candidates for the largest lead in the last maxWindow, in decreasing order
	peaks []leadAt
}

type leadAt struct {
	at   time.Time
	lead float64 // seconds of audio received beyond the wall-clock time elapsed
}

const (
	// Delivery counts as a burst while the lead grew by more than burstGrowth
	// over the last burstSpan: half a second of extra audio per second of clock.
	burstSpan   = time.Second
	burstGrowth = 0.5
	// The largest lead is taken over this window, so that a lasting change --
	// a mixer that drops audio, a clock that drifts -- is followed within it.
	maxWindow = 2 * time.Minute
	// Nothing received for this long is a new connection, not a slow network:
	// what the old one delivered says nothing about where the new one starts.
	reconnectGap = 2 * time.Second
)

func newStreamClock(sampleRate int) *streamClock {
	return &streamClock{rate: float64(sampleRate)}
}

// received records n samples read at t.
func (c *streamClock) received(n int, t time.Time) {
	if !c.started {
		c.started, c.start = true, t
	}
	if len(c.recent) > 0 && t.Sub(c.recent[len(c.recent)-1].at) > reconnectGap {
		c.recent, c.peaks = nil, nil
	}
	c.samples += int64(n)
	l := leadAt{t, float64(c.samples)/c.rate - t.Sub(c.start).Seconds()}

	c.recent = append(c.recent, l)
	for len(c.recent) > 1 && t.Sub(c.recent[1].at) >= burstSpan {
		c.recent = c.recent[1:]
	}

	for len(c.peaks) > 0 && c.peaks[len(c.peaks)-1].lead <= l.lead {
		c.peaks = c.peaks[:len(c.peaks)-1]
	}
	c.peaks = append(c.peaks, l)
	for len(c.peaks) > 1 && t.Sub(c.peaks[0].at) > maxWindow {
		c.peaks = c.peaks[1:]
	}
}

// bursting reports that the stream is running ahead of the clock.
func (c *streamClock) bursting() bool {
	if len(c.recent) < 2 {
		return true
	}
	first, last := c.recent[0], c.recent[len(c.recent)-1]
	if last.at.Sub(first.at) < burstSpan*9/10 {
		return true // not enough history yet to tell
	}
	return last.lead-first.lead > burstGrowth
}

// settled reports that the burst is over, so that the dates of what was
// marked are known.
func (c *streamClock) settled() bool { return c.started && !c.bursting() }

// mark notes where the stream stands now, for dating later what was just cut.
func (c *streamClock) mark(t time.Time) leadAt {
	if !c.started {
		return leadAt{at: t}
	}
	return leadAt{t, float64(c.samples)/c.rate - t.Sub(c.start).Seconds()}
}

// date returns when the audio marked by m was live on the air: its arrival,
// less how late it arrived compared with the most timely delivery seen.
func (c *streamClock) date(m leadAt) time.Time {
	if len(c.peaks) == 0 {
		return m.at
	}
	late := c.peaks[0].lead - m.lead
	if late < 0 {
		late = 0
	}
	return m.at.Add(-time.Duration(late * float64(time.Second)))
}

// replayFilter drops transmissions a reconnection delivers a second time.
//
// A new connection begins with the server's burst, which repeats the audio the
// old connection had already delivered. Dated by the clock, a repeated
// transmission lands where the first one did, no later than the last one sent;
// a new one lands at least a second after it (400 ms of speech, 600 ms of
// silence). Only a date clearly past the last one is new.
type replayFilter struct{ last time.Time }

const replayMargin = 500 * time.Millisecond

func (f *replayFilter) fresh(date time.Time) bool {
	if !f.last.IsZero() && !date.After(f.last.Add(replayMargin)) {
		return false
	}
	f.last = date
	return true
}
