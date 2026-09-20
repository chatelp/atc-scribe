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
func measureCapture(txPath, adsbPath, airlinesPath string, windowSec, minDigits, seeds, offsetHours int, verbose, strict bool, minScore float64, fuzzy bool, contextSec int, union bool) error {
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
	matcher.FuzzyDigits = fuzzy

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

	// Each run keeps its own memory of what it just matched on each frequency --
	// the real run and every control alike. Sharing one history would give the
	// controls the real run's answers, which is the one thing a control may not
	// have.
	type memory struct {
		at time.Time
		cs string
	}
	history := map[string][]memory{}
	recent := func(key string, at time.Time) []string {
		if contextSec <= 0 {
			return nil
		}
		var out []string
		w := time.Duration(contextSec) * time.Second
		for _, m := range history[key] {
			if at.Sub(m.at) <= w && at.Sub(m.at) >= 0 {
				out = append(out, m.cs)
			}
		}
		return out
	}
	remember := func(key string, at time.Time, cs string) {
		if contextSec <= 0 || cs == "" {
			return
		}
		h := append(history[key], memory{at, cs})
		if len(h) > 40 {
			h = h[len(h)-40:]
		}
		history[key] = h
	}

	// accept applies the same acceptance rule to the real fleet and to every
	// control fleet. A rule that is only applied to one side would measure the
	// rule instead of the signal.
	accept := func(res phraseology.Result, fleet []phraseology.Aircraft, ctx []string) (phraseology.Match, bool) {
		m, ok := matcher.MatchWithContext(res, fleet, ctx)
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

	// A unit is what gets one shot at the fleet. Normally that is one
	// transmission of one pass. Under -union it is one recording, carrying every
	// pass's attempt at it: the question stops being "which model is better" and
	// becomes "does a second model reach transmissions the first one misses".
	//
	// Uniting only the real run would measure the rule, not the signal: two texts
	// are two chances to hit a callsign by accident, so the controls are given
	// exactly the same two chances, at the same wrong moment.
	type unit struct {
		key   string // bucket: pass|freq, or union|freq
		freq  string
		at    time.Time
		texts []string
	}
	var units []unit

	if union {
		// Group by recording. Order is kept so the context window, which looks
		// backwards in time, still sees a sane sequence.
		index := map[string]int{}
		for _, tx := range txs {
			if tx.Texte == "" {
				continue
			}
			at, ok := toUTC(tx.Debut)
			if !ok {
				continue
			}
			id := tx.Freq + "|" + tx.Fichier
			if i, seen := index[id]; seen {
				units[i].texts = append(units[i].texts, tx.Texte)
				continue
			}
			index[id] = len(units)
			units = append(units, unit{key: "union|" + tx.Freq, freq: tx.Freq, at: at,
				texts: []string{tx.Texte}})
		}
		sort.SliceStable(units, func(i, j int) bool { return units[i].at.Before(units[j].at) })
	} else {
		for _, tx := range txs {
			if tx.Texte == "" {
				continue
			}
			at, ok := toUTC(tx.Debut)
			if !ok {
				continue
			}
			units = append(units, unit{key: tx.Passe + "|" + tx.Freq, freq: tx.Freq, at: at,
				texts: []string{tx.Texte}})
		}
	}

	// tryAll returns the first accepted match among a unit's texts. First, not
	// best: a rule that picked the highest score would need a way to arbitrate
	// between models, which is a product decision and not a measurement.
	tryAll := func(texts []string, fleet []phraseology.Aircraft, ctx []string) (phraseology.Match, string, bool, bool) {
		candidate := false
		for _, t := range texts {
			res := phraseology.Parse(t)
			here := false
			for _, v := range res.Values {
				if v.Role == phraseology.RoleCallsign {
					here, candidate = true, true
					break
				}
			}
			if !here {
				continue
			}
			if m, ok := accept(res, fleet, ctx); ok {
				return m, t, true, true
			}
		}
		return phraseology.Match{}, "", candidate, false
	}

	for i, u := range units {
		b := get(real, u.key)
		b.total++

		m, text, candidate, matched := tryAll(u.texts, fleetAt(u.at), recent("real|"+u.key, u.at))
		if !candidate {
			continue
		}
		b.candidates++

		if matched {
			remember("real|"+u.key, u.at, m.Callsign)
			b.matched++
			if verbose {
				fmt.Printf("  %s %s %-9s %.2f %-34s %s\n", u.freq, u.at.Format("15:04:05"),
					m.Callsign, m.Score, m.Reason, trunc(text, 58))
			}
		}
		for s := 1; s <= seeds; s++ {
			cb := get(ctrl, u.key)
			if s == 1 {
				cb.candidates++
			}
			ck := fmt.Sprintf("ctrl%d|%s", s, u.key)
			cm, _, _, cok := tryAll(u.texts, fleetAt(pick(s, i)), recent(ck, u.at))
			if cok {
				remember(ck, u.at, cm.Callsign)
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
