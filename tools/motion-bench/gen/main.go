// Command gen serves synthetic traffic the way tar1090 serves a receiver's,
// in real time, for the browser bench (tools/motion-bench/bench.mjs,
// docs-fr/fiches-cloud/M-mouvement-ecran.md).
//
//	go run ./tools/motion-bench/gen -addr 127.0.0.1:8790 -aircraft 60
//
// GET /data/aircraft.json  the fleet as readsb writes it, rewritten once a
//
//	second (internal/adsb/simtraffic: 0.6 positions a
//	second and per aircraft, seen_pos growing, 30 s
//	losses, the long gap of 18 s);
//
// GET /data/receiver.json, /data/stats.json  what tar1090 serves beside it;
// GET /truth?from=MS&to=MS&step=MS  where each aircraft truly was, by wall
//
//	clock (ms since the epoch): the page never sees it.
//
// The place is the made-up receiver of the replay bench, 45.0 / 4.67.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/yegors/co-atc/internal/adsb/simtraffic"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8790", "where to serve")
	n := flag.Int("aircraft", 60, "aircraft flown")
	seconds := flag.Float64("seconds", 1500, "seconds of flight; the file stops changing after")
	seed := flag.Int64("seed", 20261013, "seed of every random draw")
	longGap := flag.Float64("long-gap-from", 150, "second the long gap (18 s without a position) starts, 0 for none")
	lat := flag.Float64("lat", 45.0, "receiver latitude")
	lon := flag.Float64("lon", 4.67, "receiver longitude")
	flag.Parse()

	built := time.Now()
	traffic := simtraffic.New(simtraffic.Options{Aircraft: *n, Seconds: *seconds, Seed: *seed,
		StationLat: *lat, StationLon: *lon, LongGapFrom: *longGap, StoreEvery: 10})
	// The flight starts on the next whole second, once the paths are flown.
	start := time.Now().Truncate(time.Second).Add(time.Second)
	log.Printf("%d aircraft flown in %v; t=0 at %s", len(traffic.Fleet), time.Since(built).Round(time.Millisecond), start.Format(time.RFC3339Nano))

	var current atomic.Pointer[[]byte]
	write := func(k int) {
		body, err := traffic.AircraftJSON(start, float64(k), k*1000*len(traffic.Fleet)/12)
		if err != nil {
			log.Fatal(err)
		}
		current.Store(&body)
	}
	write(0)
	go func() {
		time.Sleep(time.Until(start))
		for k := 0; float64(k) <= *seconds; k++ {
			write(k)
			time.Sleep(time.Until(start.Add(time.Duration(k+1) * time.Second)))
		}
	}()

	mux := http.NewServeMux()
	serveJSON := func(w http.ResponseWriter, body []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(body)
	}
	mux.HandleFunc("/data/aircraft.json", func(w http.ResponseWriter, r *http.Request) { serveJSON(w, *current.Load()) })
	mux.HandleFunc("/data/receiver.json", func(w http.ResponseWriter, r *http.Request) {
		serveJSON(w, []byte(fmt.Sprintf(`{"version":"motion-bench","refresh":1000,"history":0,"lat":%g,"lon":%g}`, *lat, *lon)))
	})
	mux.HandleFunc("/data/stats.json", func(w http.ResponseWriter, r *http.Request) {
		serveJSON(w, []byte(fmt.Sprintf(`{"now":%d,"aircraft_with_pos":%d}`, time.Now().Unix(), len(traffic.Fleet))))
	})
	mux.HandleFunc("/truth", func(w http.ResponseWriter, r *http.Request) {
		startMs := start.UnixMilli()
		q := r.URL.Query()
		from, _ := strconv.ParseInt(q.Get("from"), 10, 64)
		to, _ := strconv.ParseInt(q.Get("to"), 10, 64)
		step, _ := strconv.ParseInt(q.Get("step"), 10, 64)
		if step <= 0 {
			step = 100
		}
		if to <= 0 {
			to = time.Now().UnixMilli()
		}
		from = max(from, startMs)
		type truth struct {
			Hex    string       `json:"hex"`
			Kind   string       `json:"kind"`
			GapMs  [2]int64     `json:"gap_ms"` // wall clock, 0 0 for none
			Points [][5]float64 `json:"points"` // lat, lon, track deg, ground speed m/s, alt ft; every step from `from`
		}
		out := struct {
			Start  int64   `json:"start_ms"`
			From   int64   `json:"from_ms"`
			StepMs int64   `json:"step_ms"`
			Truth  []truth `json:"truth"`
		}{Start: startMs, From: from, StepMs: step}
		for _, a := range traffic.Fleet {
			tr := truth{Hex: a.Hex, Kind: a.Kind}
			if a.GapTo > 0 {
				tr.GapMs = [2]int64{startMs + int64(a.GapFrom*1000), startMs + int64(a.GapTo*1000)}
			}
			for ms := from; ms <= to; ms += step {
				p := a.TruthAt(math.Min(*seconds, float64(ms-startMs)/1000))
				tr.Points = append(tr.Points, [5]float64{simtraffic.Round(p.Lat, 7), simtraffic.Round(p.Lon, 7),
					simtraffic.Round(p.TrackDeg, 2), simtraffic.Round(p.GsKt*simtraffic.KtToMps, 2), math.Round(p.AltFt)})
			}
			out.Truth = append(out.Truth, tr)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	log.Printf("serving on http://%s/data/", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
