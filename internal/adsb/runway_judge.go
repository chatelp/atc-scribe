package adsb

import (
	"math"
	"strings"
	"sync"
	"time"
)

// RunwayAssignment is the runway an aircraft landed or took off on, as judged
// from its track: the airport, the runway end ("09L") and when it was judged.
type RunwayAssignment struct {
	Airport string    `json:"airport"`
	Runway  string    `json:"runway"`
	Since   time.Time `json:"since"`
}

// AircraftRunways is what the runway judge says of one aircraft. Both can be
// set: a local flight takes off and lands; each is kept for the rest of the
// flight.
type AircraftRunways struct {
	Arrival   *RunwayAssignment `json:"arrival,omitempty"`
	Departure *RunwayAssignment `json:"departure,omitempty"`
}

// The runway judge is the one place that decides which runway an aircraft uses,
// so that the details panel, the PAR view and the runway-in-use summary cannot
// disagree (docs-fr/05-decisions.md, D69).
//
// An aircraft is established on a runway when it is within establishedOffsetNM
// of the extended centreline, within establishedRangeNM of the threshold, its
// track within establishedTrackDeg of the runway's, below the approach ceiling.
// Its arrival runway is the last one it was established on, so a late change of
// runway is followed. Measured against FlightAware's actual landing runway over
// six days (tools/runway-check): 1504 arrivals right of 1505 at Paris-CDG, 449 of
// 449 at Orly. 185 m stays under half the 383 m between CDG's parallels.
const (
	establishedOffsetNM  = 0.1
	establishedRangeNM   = 8.0
	establishedTrackDeg  = 15.0
	departureRangeNM     = 5.0
	departureMinClimbFPM = 300.0
	// A climb straight out along the runway just after an approach to it is a
	// go-around, not a take-off.
	goAroundWindow = 15 * time.Minute
	// An aircraft that lands and leaves again is on a new flight: its arrival
	// is forgotten once it is judged departing that long after.
	turnaroundGap = 20 * time.Minute
	// Kept this long after the aircraft was last seen, as long as the list and
	// the details panel may still show it.
	runwayJudgeExpiry = 3 * time.Hour
)

// RunwayObservation is what the judge needs of one position report.
type RunwayObservation struct {
	Lat, Lon      float64
	TrackDeg      float64
	AltFt         float64
	BaroRateFPM   *float64
	OnGround      bool
	MaxApproachFt float64
	At            time.Time
}

type runwayJudgeEntry struct {
	runways  AircraftRunways
	lastSeen time.Time
}

// RunwayJudge remembers the runways of each aircraft, keyed by hex.
type RunwayJudge struct {
	mu          sync.Mutex
	entries     map[string]*runwayJudgeEntry
	lastCleanup time.Time
}

func NewRunwayJudge() *RunwayJudge {
	return &RunwayJudge{entries: make(map[string]*runwayJudgeEntry)}
}

// runwayEndGeometry places a position relative to a runway end, flat-earth in
// NM: dist before the threshold along the landing direction (positive on the
// approach side), offset from the centreline, and the landing direction.
func runwayEndGeometry(endLat, endLon, otherLat, otherLon, lat, lon float64) (along, cross, bearing, length float64) {
	cosLat := math.Cos(endLat * math.Pi / 180)
	ox, oy := (otherLon-endLon)*60*cosLat, (otherLat-endLat)*60
	x, y := (lon-endLon)*60*cosLat, (lat-endLat)*60
	b := math.Atan2(ox, oy)
	along = x*math.Sin(b) + y*math.Cos(b)
	cross = x*math.Cos(b) - y*math.Sin(b)
	return along, cross, math.Mod(b*180/math.Pi+360, 360), math.Hypot(ox, oy)
}

// established finds the runway end of the followed airports an aircraft is
// established on, arriving (on final before the threshold) or departing (in
// the initial climb past the far end), nearest centreline first.
func established(refs []PhaseReference, o RunwayObservation, departing bool) (airport, end string, ok bool) {
	best := math.Inf(1)
	for _, ref := range refs {
		for pair, thresholds := range ref.Runways.RunwayThresholds {
			for endID, t := range thresholds {
				other, found := thresholds[getOppositeThreshold(endID, pair)]
				if !found {
					continue
				}
				along, cross, bearing, length := runwayEndGeometry(t.Latitude, t.Longitude, other.Latitude, other.Longitude, o.Lat, o.Lon)
				var d float64
				if departing {
					d = along - length // past the far end, flying the take-off direction
					if d <= 0 || d > departureRangeNM {
						continue
					}
				} else {
					d = -along // before the threshold
					if d <= 0 || d > establishedRangeNM {
						continue
					}
				}
				if math.Abs(cross) > establishedOffsetNM || angleDiffDeg(o.TrackDeg, bearing) > establishedTrackDeg {
					continue
				}
				if math.Abs(cross) < best {
					best, airport, end, ok = math.Abs(cross), ref.Airport, endID, true
				}
			}
		}
	}
	return airport, end, ok
}

func angleDiffDeg(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// Observe judges one position report of an aircraft against the followed
// airports.
func (j *RunwayJudge) Observe(hex string, refs []PhaseReference, o RunwayObservation) {
	if j == nil || hex == "" {
		return
	}
	hex = strings.ToLower(hex)
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cleanupLocked(o.At)

	e := j.entries[hex]
	if e == nil {
		e = &runwayJudgeEntry{}
		j.entries[hex] = e
	}
	e.lastSeen = o.At
	if o.OnGround || o.AltFt <= 0 || (o.MaxApproachFt > 0 && o.AltFt > o.MaxApproachFt) {
		return
	}

	if ap, end, ok := established(refs, o, false); ok {
		arr := e.runways.Arrival
		if arr == nil || arr.Airport != ap || arr.Runway != end {
			e.runways.Arrival = &RunwayAssignment{Airport: ap, Runway: end, Since: o.At}
		}
		return
	}

	climbing := o.BaroRateFPM != nil && *o.BaroRateFPM >= departureMinClimbFPM
	if !climbing || e.runways.Departure != nil {
		return
	}
	if ap, end, ok := established(refs, o, true); ok {
		if arr := e.runways.Arrival; arr != nil && arr.Airport == ap {
			if o.At.Sub(arr.Since) < goAroundWindow {
				return
			}
			if o.At.Sub(arr.Since) >= turnaroundGap {
				e.runways.Arrival = nil
			}
		}
		e.runways.Departure = &RunwayAssignment{Airport: ap, Runway: end, Since: o.At}
	}
}

// Get returns a copy of what the judge says of an aircraft, nil when nothing.
func (j *RunwayJudge) Get(hex string) *AircraftRunways {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	e := j.entries[strings.ToLower(hex)]
	if e == nil || (e.runways.Arrival == nil && e.runways.Departure == nil) {
		return nil
	}
	out := &AircraftRunways{}
	if a := e.runways.Arrival; a != nil {
		c := *a
		out.Arrival = &c
	}
	if d := e.runways.Departure; d != nil {
		c := *d
		out.Departure = &c
	}
	return out
}

func (j *RunwayJudge) cleanupLocked(now time.Time) {
	if now.Sub(j.lastCleanup) < time.Minute {
		return
	}
	j.lastCleanup = now
	for hex, e := range j.entries {
		if now.Sub(e.lastSeen) > runwayJudgeExpiry {
			delete(j.entries, hex)
		}
	}
}

func aircraftRunwaysEqual(a, b *AircraftRunways) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	same := func(x, y *RunwayAssignment) bool {
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return x.Airport == y.Airport && x.Runway == y.Runway
	}
	return same(a.Arrival, b.Arrival) && same(a.Departure, b.Departure)
}
