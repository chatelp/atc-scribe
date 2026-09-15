// Command phraseology runs the phraseology grammar over a file of transcripts and
// reports what it could extract. It exists to measure the grammar against real
// corpora rather than against the examples it was written from.
//
//	go run ./cmd/phraseology -in transcripts.json [-field texte]
//
// Input is a JSON array of objects; -field names the key holding the text.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/yegors/co-atc/internal/transcription/phraseology"
)

func main() {
	in := flag.String("in", "", "JSON array of objects holding transcripts")
	field := flag.String("field", "texte", "object key holding the text")
	verbose := flag.Bool("v", false, "print every parsed transmission")
	flag.Parse()

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
