package transcription

import (
	"sync/atomic"

	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/transcription/phraseology"
)

// The procedures heard, named from the station's list of official names
// (internal/aip; phraseology/places.go; Q55). One namer for the API and the
// processor's broadcasts, set at start and again when a list is made from the
// settings page; the sound likeness asked for is read each time, so the
// panel's setting applies to the next transmission served.

type placeConfig struct {
	namer *phraseology.PlaceNamer
	floor func() (float64, bool)
}

var places atomic.Pointer[placeConfig]

// SetPlaces gives the namer and how to read the setting in force.
func SetPlaces(n *phraseology.PlaceNamer, floor func() (float64, bool)) {
	places.Store(&placeConfig{namer: n, floor: floor})
}

// PlacesOf names the procedures heard in a transmission's text as received,
// with where their words are in it and in its processed form.
func PlacesOf(content string) []sqlite.PlaceName {
	pc := places.Load()
	if pc == nil || pc.namer == nil || pc.floor == nil || content == "" {
		return nil
	}
	floor, on := pc.floor()
	if !on || pc.namer.Count() == 0 {
		return nil
	}
	r := phraseology.Parse(content)
	var out []sqlite.PlaceName
	for _, p := range pc.namer.Name(r, floor) {
		pn := sqlite.PlaceName{Designator: p.Designator, Point: p.Point, Lat: p.Lat, Lon: p.Lon, Score: p.Score}
		if s := r.RawSpans([][2]int{p.Words}); len(s) == 1 {
			pn.Raw = []int{s[0][0], s[0][1]}
		}
		if s := r.NormalizedSpans([][2]int{p.Words}); len(s) == 1 {
			pn.Normalized = []int{s[0][0], s[0][1]}
		}
		out = append(out, pn)
	}
	return out
}
