package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/yegors/co-atc/internal/transcription/phraseology"
)

// A frequency's sector: the aircraft it can plausibly be talking to. Over
// Paris the matcher compares a transmission with a hundred aircraft, of which
// a De Gaulle approach frequency can address a quarter; the rest only add
// chance matches (docs-fr/05-decisions.md, Q46, track 3).
type sector struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	RadiusNM float64 `json:"radius_nm"`
	MaxAltFt float64 `json:"max_alt_ft"`
}

type fix struct {
	T        int64
	Lat, Lon float64
}

// loadSectors returns a filter keeping the aircraft inside a frequency's
// sector, or nil when no sectors are given. An aircraft with no known position
// near that moment is kept: the filter only removes what it knows to be out.
func loadSectors(positionsPath, sectorsPath string) (func(string, phraseology.Aircraft, time.Time) bool, error) {
	if sectorsPath == "" {
		return nil, nil
	}
	var sectors map[string]sector
	b, err := os.ReadFile(sectorsPath)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &sectors); err != nil {
		return nil, fmt.Errorf("sectors: %w", err)
	}
	var raw []struct {
		T   int64   `json:"t"`
		Hex string  `json:"hex"`
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	if positionsPath != "" {
		b, err := os.ReadFile(positionsPath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, fmt.Errorf("positions: %w", err)
		}
	}
	byHex := map[string][]fix{}
	for _, p := range raw {
		byHex[p.Hex] = append(byHex[p.Hex], fix{p.T, p.Lat, p.Lon})
	}
	for _, v := range byHex {
		sort.Slice(v, func(i, j int) bool { return v[i].T < v[j].T })
	}
	return func(freq string, a phraseology.Aircraft, at time.Time) bool {
		sc, ok := sectors[freq]
		if !ok {
			return true
		}
		if sc.MaxAltFt > 0 && a.AltitudeFt > sc.MaxAltFt {
			return false
		}
		v := byHex[a.Hex]
		i := sort.Search(len(v), func(i int) bool { return v[i].T >= at.Unix() })
		best, gap := fix{}, int64(math.MaxInt64)
		for _, j := range []int{i - 1, i} {
			if j >= 0 && j < len(v) {
				if d := abs64(v[j].T - at.Unix()); d < gap {
					best, gap = v[j], d
				}
			}
		}
		if gap > 180 {
			return true
		}
		return nm(sc.Lat, sc.Lon, best.Lat, best.Lon) <= sc.RadiusNM
	}, nil
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func nm(lat1, lon1, lat2, lon2 float64) float64 {
	r := math.Pi / 180
	a := math.Sin((lat2-lat1)*r/2)*math.Sin((lat2-lat1)*r/2) +
		math.Cos(lat1*r)*math.Cos(lat2*r)*math.Sin((lon2-lon1)*r/2)*math.Sin((lon2-lon1)*r/2)
	return 3440.1 * 2 * math.Asin(math.Sqrt(a))
}
