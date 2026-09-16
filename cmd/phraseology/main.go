// Command phraseology runs the phraseology grammar over a file of transcripts and
// reports what it could extract. It exists to measure the grammar against real
// corpora rather than against the examples it was written from.
//
//	go run ./cmd/phraseology -in transcripts.json [-field texte]
//
// Input is a JSON array of objects; -field names the key holding the text.
//
// With -db it instead reads a co-atc daily database and measures how often a
// transmission can be attached to an aircraft the receiver was actually seeing
// at that moment. That number is a correctness signal that costs no annotation:
// a spoken flight number matching a real target in the sky is not a coincidence.
//
//	go run ./cmd/phraseology -db data/co-atc-2026-09-15.db [-window 120]
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/yegors/co-atc/internal/transcription/phraseology"
	_ "modernc.org/sqlite"
)

func main() {
	in := flag.String("in", "", "JSON array of objects holding transcripts")
	db := flag.String("db", "", "co-atc daily SQLite database to measure against")
	window := flag.Int("window", 120, "seconds either side of a transmission to look for aircraft")
	control := flag.Int("control", 0, "shift the aircraft window by this many seconds: a crude null hypothesis")
	shuffle := flag.Int64("shuffle", 0, "seed: draw each fleet from a random other moment — the real null hypothesis")
	airlines := flag.String("airlines", "assets/airlines.dat", "OpenFlights airlines.dat")
	minDigits := flag.Int("min-digits", 3, "shortest spoken number that may be a flight number")
	capture := flag.String("capture", "", "transcripts of a recorded capture (JSON)")
	adsb := flag.String("adsb", "", "ADS-B history recovered from readsb heatmaps (JSON)")
	seeds := flag.Int("seeds", 5, "control runs")
	offset := flag.Int("offset-hours", 2, "hours to subtract from capture times to reach UTC")
	field := flag.String("field", "texte", "object key holding the text")
	verbose := flag.Bool("v", false, "print every parsed transmission")
	strict := flag.Bool("strict", false, "refuse a match when a second aircraft scores nearly as well")
	minScore := flag.Float64("min-score", 0, "refuse a match below this score")
	flag.Parse()

	if *capture != "" {
		if err := measureCapture(*capture, *adsb, *airlines, *window, *minDigits, *seeds, *offset, *verbose, *strict, *minScore); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if *db != "" {
		if err := measureAgainstADSB(*db, *airlines, *window, *control, *shuffle, *minDigits, *verbose); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	raw, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	byRole := map[phraseology.Role]map[string]int{}
	speakers := map[phraseology.Speaker]int{}
	var withValue, clearances int

	for _, r := range rows {
		text, _ := r[*field].(string)
		if text == "" {
			continue
		}
		res := phraseology.Parse(text)
		if len(res.Values) > 0 {
			withValue++
		}
		clearances += len(res.Clearances)
		speakers[res.Speaker]++
		for _, v := range res.Values {
			if byRole[v.Role] == nil {
				byRole[v.Role] = map[string]int{}
			}
			byRole[v.Role][v.Text]++
		}
		if *verbose {
			fmt.Printf("%-70s", trunc(text, 70))
			for _, v := range res.Values {
				fmt.Printf(" [%s=%s]", v.Role, v.Text)
			}
			for _, l := range res.Letters {
				fmt.Printf(" [letters=%s]", l)
			}
			fmt.Println()
		}
	}

	fmt.Printf("\n%d transmissions, %d with at least one value, %d clearances\n",
		len(rows), withValue, clearances)
	fmt.Printf("speaker: ATC=%d PILOT=%d unknown=%d\n\n",
		speakers[phraseology.SpeakerATC], speakers[phraseology.SpeakerPilot],
		speakers[phraseology.SpeakerUnknown])

	roles := make([]phraseology.Role, 0, len(byRole))
	for r := range byRole {
		roles = append(roles, r)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })
	for _, role := range roles {
		vals := byRole[role]
		type kv struct {
			k string
			n int
		}
		list := make([]kv, 0, len(vals))
		total := 0
		for k, n := range vals {
			list = append(list, kv{k, n})
			total += n
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].n != list[j].n {
				return list[i].n > list[j].n
			}
			return list[i].k < list[j].k
		})
		fmt.Printf("%-14s %3d occurrences:", role, total)
		for i, e := range list {
			if i >= 12 {
				fmt.Printf(" …(+%d)", len(list)-12)
				break
			}
			fmt.Printf(" %s×%d", e.k, e.n)
		}
		fmt.Println()
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ---------------------------------------------------------------- ADS-B measure

type transmission struct {
	at   time.Time
	text string
	freq string
}

// measureAgainstADSB is the only correctness measure in this project that needs
// no ground truth. For each transmission it asks: of the aircraft the receiver
// could see at that moment, does any one of them carry the flight number that was
// spoken? Chance alone would rarely say yes.
// A control shift answers the only question that makes the match rate meaningful:
// how often would a transmission attach to an aircraft that was nowhere near, at a
// time it could not have been talking to? Comparing the real rate against that
// shifted rate separates signal from arithmetic.
func measureAgainstADSB(dbPath, airlinesPath string, windowSec, controlShift int, shuffleSeed int64, minDigits int, verbose bool) error {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	matcher, err := phraseology.NewMatcher(airlinesPath)
	if err != nil {
		return fmt.Errorf("airlines.dat: %w", err)
	}
	matcher.MinDigits = minDigits

	// Every ADS-B sighting, sorted, so each transmission can binary-search its
	// own moment instead of re-querying 300 times over 300k rows.
	type sighting struct {
		at     time.Time
		flight string
		hex    string
		alt    float64
	}
	var sky []sighting
	rows, err := conn.Query(`SELECT timestamp, flight, hex, COALESCE(alt_baro, 0) FROM adsb_targets
	                          WHERE flight IS NOT NULL AND TRIM(flight) != ''`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var ts, fl, hx string
		var alt float64
		if err := rows.Scan(&ts, &fl, &hx, &alt); err != nil {
			return err
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		sky = append(sky, sighting{t, strings.TrimSpace(fl), hx, alt})
	}
	rows.Close()
	sort.Slice(sky, func(i, j int) bool { return sky[i].at.Before(sky[j].at) })

	var txs []transmission
	rows, err = conn.Query(`SELECT created_at, content, frequency_id FROM transcriptions
	                         WHERE content != '' ORDER BY created_at`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var ts, txt, fq string
		if err := rows.Scan(&ts, &txt, &fq); err != nil {
			return err
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		txs = append(txs, transmission{t, txt, fq})
	}
	rows.Close()

	if len(sky) == 0 {
		return fmt.Errorf("no ADS-B sightings in %s", dbPath)
	}
	fmt.Printf("%d transmissions, %d ADS-B sightings, window ±%ds\n",
		len(txs), len(sky), windowSec)
	fmt.Printf("ADS-B covers %s → %s\n\n",
		sky[0].at.UTC().Format("15:04:05Z"), sky[len(sky)-1].at.UTC().Format("15:04:05Z"))

	w := time.Duration(windowSec) * time.Second
	var covered, withCandidate, matched, ambiguous int
	var fleetSizes []int

	shift := time.Duration(controlShift) * time.Second
	if controlShift != 0 {
		fmt.Printf("CONTROL: aircraft taken from %+ds away, where none of them can be the speaker\n\n", controlShift)
	}

	// A shifted window falls off the end of the recording and quietly shrinks the
	// sample. Drawing each fleet from a random other moment inside the covered
	// span keeps all the transmissions and all the fleet sizes, and changes only
	// the one thing under test: whether the aircraft could have been speaking.
	rng := rand.New(rand.NewSource(shuffleSeed))
	if shuffleSeed != 0 {
		fmt.Printf("CONTROL: each fleet drawn from a random other moment (seed %d)\n\n", shuffleSeed)
	}

	for _, tx := range txs {
		at := tx.at.Add(shift)
		if shuffleSeed != 0 {
			// Draw the moment from the sightings themselves, not from the clock:
			// the recording has gaps, and a uniform draw over the span lands in one
			// two times out of three, silently shrinking the control sample.
			for i := 0; i < 40; i++ {
				cand := sky[rng.Intn(len(sky))].at
				if cand.Sub(tx.at).Abs() > 10*time.Minute {
					at = cand
					break
				}
			}
		}
		lo := sort.Search(len(sky), func(i int) bool { return !sky[i].at.Before(at.Add(-w)) })
		hi := sort.Search(len(sky), func(i int) bool { return sky[i].at.After(at.Add(w)) })
		if lo >= hi {
			continue // no ADS-B coverage at this moment
		}
		covered++

		// The last sighting inside the window is the aircraft's state at that moment.
		seen := map[string]phraseology.Aircraft{}
		for _, s := range sky[lo:hi] {
			seen[s.flight] = phraseology.Aircraft{Callsign: s.flight, Hex: s.hex, AltitudeFt: s.alt}
		}
		fleet := make([]phraseology.Aircraft, 0, len(seen))
		for _, ac := range seen {
			fleet = append(fleet, ac)
		}
		fleetSizes = append(fleetSizes, len(fleet))

		res := phraseology.Parse(tx.text)
		hasCandidate := false
		for _, v := range res.Values {
			if v.Role == phraseology.RoleCallsign {
				hasCandidate = true
			}
		}
		if !hasCandidate {
			continue
		}
		withCandidate++

		m, ok := matcher.Match(res, fleet)
		if !ok {
			if verbose {
				fmt.Printf("  %s  —        %s\n", tx.at.Format("15:04:05"), trunc(tx.text, 80))
			}
			continue
		}
		matched++
		if m.Ambiguous {
			ambiguous++
		}
		if verbose {
			flag := ""
			if m.Ambiguous {
				flag = fmt.Sprintf(" (ambiguous with %s)", strings.Join(m.Runners, ", "))
			}
			fmt.Printf("  %s  %-9s %.2f %-22s %s%s\n", tx.at.Format("15:04:05"),
				m.Callsign, m.Score, m.Reason, trunc(tx.text, 60), flag)
		}
	}

	sort.Ints(fleetSizes)
	median := 0
	if len(fleetSizes) > 0 {
		median = fleetSizes[len(fleetSizes)/2]
	}
	fmt.Printf("\nwith ADS-B coverage        %d\n", covered)
	fmt.Printf("with a callsign candidate  %d  (%.0f%% of covered)\n",
		withCandidate, pct(withCandidate, covered))
	fmt.Printf("attached to an aircraft    %d  (%.0f%% of candidates)\n",
		matched, pct(matched, withCandidate))
	fmt.Printf("  of which ambiguous       %d\n", ambiguous)
	fmt.Printf("median aircraft in window  %d\n", median)
	return nil
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}
