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

// phaseRef holds the airports behind an atomic pointer, the principal first. They
// can be changed from the settings panel while the fetch goroutine is
// classifying aircraft, and a phase computed half against one set and half
// against another would be worse than either. Readers load once and use that
// copy throughout.
//
// Several airports can be followed at once -- Orly and De Gaulle, 13 NM apart,
// seen from one receiver. Each aircraft is judged against one of them, the one
// it is flying to or from (airportFor); with a single airport, everything is as
// before.
type phaseRef struct {
	p atomic.Pointer[[]PhaseReference]
}

func newPhaseRef(initial PhaseReference) *phaseRef {
	r := &phaseRef{}
	r.store(initial)
	return r
}

// load returns the principal airport.
func (r *phaseRef) load() *PhaseReference {
	if v := r.p.Load(); v != nil && len(*v) > 0 {
		return &(*v)[0]
	}
	return &PhaseReference{}
}

// all returns every airport followed, the principal first. Never empty.
func (r *phaseRef) all() []PhaseReference {
	if v := r.p.Load(); v != nil && len(*v) > 0 {
		return *v
	}
	return []PhaseReference{{}}
}

func (r *phaseRef) store(v PhaseReference) {
	r.storeAll([]PhaseReference{v})
}

func (r *phaseRef) storeAll(v []PhaseReference) {
	c := append([]PhaseReference(nil), v...)
	r.p.Store(&c)
}
