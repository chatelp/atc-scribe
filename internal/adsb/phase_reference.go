package adsb

import "sync/atomic"

// PhaseReference is the airport that approach, departure, takeoff and landing
// are judged against.
//
// Upstream measured all of these from the receiver's own position. That is right
// when the receiver sits on its airport -- the configuration's example is Toronto
// Pearson -- and wrong otherwise: its own approach rule is commented "verify
// heading toward airport" over code that reads the station. On a receiver 14 NM
// from the airport it watches, no departure ever reached CLB, an approach counted
// only if it happened to be flown towards the receiver, and aircraft on the
// airport's ground were filtered out as too far away.
//
// The receiver's position keeps its own meaning -- the distance on screen, the
// range rings, the predicted positions -- and stays in the service as before.
// When no airport position is known the reference falls back to the receiver,
// which is exactly upstream's behaviour.
type PhaseReference struct {
	Airport string // ICAO code, "" when none is configured
	Lat     float64
	Lon     float64
	Runways RunwayData
}

// phaseRef holds the reference behind an atomic pointer. It can be changed from
// the settings panel while the fetch goroutine is classifying aircraft, and a
// phase computed half against one airport and half against another would be
// worse than either. Readers load it once and use that copy throughout.
type phaseRef struct {
	p atomic.Pointer[PhaseReference]
}

func newPhaseRef(initial PhaseReference) *phaseRef {
	r := &phaseRef{}
	r.store(initial)
	return r
}

func (r *phaseRef) load() *PhaseReference {
	if v := r.p.Load(); v != nil {
		return v
	}
	return &PhaseReference{}
}

func (r *phaseRef) store(v PhaseReference) {
	r.p.Store(&v)
}
