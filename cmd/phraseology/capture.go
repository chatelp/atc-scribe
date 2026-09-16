package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/yegors/co-atc/internal/transcription/phraseology"
)

// This mode measures a recorded capture rather than a live database: transcripts
// on one side, ADS-B history on the other, matched per frequency.
//
// Per frequency matters. The earlier figure came from the mixed stream of five
// channels, so it averaged an approach sector, an en-route sector and a bilingual
// tower together. Separated, each frequency can be judged on its own.

type captureTx struct {
	Passe   string  `json:"passe"`
	Freq    string  `json:"freq"`
	Fichier string  `json:"fichier"`
	Debut   string  `json:"debut"` // local station time, no zone
	Duree   float64 `json:"duree"`
	Texte   string  `json:"texte"`
}

type adsbPoint struct {
	T      int64   `json:"t"` // unix seconds, UTC
	Hex    string  `json:"hex"`
	Flight string  `json:"flight"`
	Alt    float64 `json:"alt"`
}

func loadJSON[T any](path string) ([]T, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []T
	return out, json.Unmarshal(raw, &out)
}

// measureCapture cross-checks a capture against recovered ADS-B history.
func measureCapture(txPath, adsbPath, airlinesPath string, windowSec, minDigits, seeds, offsetHours int, verbose, strict bool, minScore float64) error {
	txs, err := loadJSON[captureTx](txPath)
	if err != nil {
		return fmt.Errorf("transcripts: %w", err)
	}
	sky, err := loadJSON[adsbPoint](adsbPath)
	if err != nil {
		return fmt.Errorf("adsb: %w", err)
	}
	sort.Slice(sky, func(i, j int) bool { return sky[i].T < sky[j].T })

	matcher, err := phraseology.NewMatcher(airlinesPath)
	if err != nil {
		return err
	}
	matcher.MinDigits = minDigits

	// Capture filenames carry station local time; ADS-B carries UTC.
	toUTC := func(local string) (time.Time, bool) {
		t, err := time.Parse("2006-01-02T15:04:05", local)
		if err != nil {
			return time.Time{}, false
		}
		return t.Add(-time.Duration(offsetHours) * time.Hour).UTC(), true
	}

	fleetAt := func(at time.Time) []phraseology.Aircraft {
		lo := sort.Search(len(sky), func(i int) bool { return sky[i].T >= at.Unix()-int64(windowSec) })
		hi := sort.Search(len(sky), func(i int) bool { return sky[i].T > at.Unix()+int64(windowSec) })
		seen := map[string]phraseology.Aircraft{}
		for _, p := range sky[lo:hi] {
			if p.Flight == "" {
				continue
			}
			seen[p.Flight] = phraseology.Aircraft{Callsign: p.Flight, Hex: p.Hex, AltitudeFt: p.Alt}
		}
		out := make([]phraseology.Aircraft, 0, len(seen))
		for _, a := range seen {
			out = append(out, a)
		}
		return out
	}

	type bucket struct{ total, candidates, matched int }
	real := map[string]*bucket{}
	ctrl := map[string]*bucket{}
	get := func(m map[string]*bucket, k string) *bucket {
		if m[k] == nil {
			m[k] = &bucket{}
		}
		return m[k]
	}

	// Control times are drawn from the sightings themselves so every transmission
	// keeps a fleet of realistic size; only the moment is wrong.
	pick := func(seed, i int) time.Time {
		return time.Unix(sky[(seed*7919+i*104729)%len(sky)].T, 0).UTC()
	}

	// accept applies the same acceptance rule to the real fleet and to every
	// control fleet. A rule that is only applied to one side would measure the
	// rule instead of the signal.
	accept := func(res phraseology.Result, fleet []phraseology.Aircraft) (phraseology.Match, bool) {
		m, ok := matcher.Match(res, fleet)
		if !ok {
			return m, false
		}
		if strict && m.Ambiguous {
			return m, false
		}
		if m.Score < minScore {
			return m, false
		}
		return m, true
	}

	for i, tx := range txs {
		if tx.Texte == "" {
			continue
		}
		at, ok := toUTC(tx.Debut)
		if !ok {
			continue
		}
		key := tx.Passe + "|" + tx.Freq
		b := get(real, key)
		b.total++

		res := phraseology.Parse(tx.Texte)
		has := false
		for _, v := range res.Values {
			if v.Role == phraseology.RoleCallsign {
				has = true
			}
		}
		if !has {
			continue
		}
		b.candidates++

		if m, ok := accept(res, fleetAt(at)); ok {
			b.matched++
			if verbose {
				fmt.Printf("  %s %s %-9s %.2f %-34s %s\n", tx.Freq, at.Format("15:04:05"),
					m.Callsign, m.Score, m.Reason, trunc(tx.Texte, 58))
			}
		}
		for s := 1; s <= seeds; s++ {
			cb := get(ctrl, key)
			if s == 1 {
				cb.candidates++
			}
			if _, ok := accept(res, fleetAt(pick(s, i))); ok {
				cb.matched++
			}
		}
	}

	keys := make([]string, 0, len(real))
	for k := range real {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("\n%-12s %-9s %6s %6s %8s %8s %9s %8s\n",
		"passe", "freq", "trans", "cand", "apparies", "hasard", "precision", "vrais")
	fmt.Println(strings.Repeat("-", 74))
	for _, k := range keys {
		r := real[k]
		// A frequency can have transmissions and no candidate at all — at four
		// digits the French pass has none — so its control bucket is never created.
		c := ctrl[k]
		if c == nil {
			c = &bucket{}
		}
		chance := float64(c.matched) / float64(seeds)
		prec, vrais := 0.0, 0.0
		if r.matched > 0 {
			prec = 100 * (1 - chance/float64(r.matched))
			vrais = float64(r.matched) - chance
		}
		parts := strings.SplitN(k, "|", 2)
		fmt.Printf("%-12s %-9s %6d %6d %8d %8.1f %8.0f%% %8.1f\n",
			parts[0], parts[1], r.total, r.candidates, r.matched, chance, prec, vrais)
	}
	return nil
}
