package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/yegors/co-atc/internal/transcription/phraseology"
)

// -json writes one line per transmission of the real run: what it was matched
// to, and each reading laid out on its tokens with every stretch the grammar
// recognised. It serves the judges of the transcript itself rather than of the
// callsign alone (docs-fr/05-decisions.md, D63): the share of tokens outside
// every span, and the values read, checked against the ADS-B, are computed from
// it in whisper-lab.

type jsonMatch struct {
	Flight    string   `json:"flight"`
	Hex       string   `json:"hex"`
	Score     float64  `json:"score"`
	Reason    string   `json:"reason"`
	Ambiguous bool     `json:"ambiguous"`
	Runners   []string `json:"runners,omitempty"`
	// Refused is set when the matcher found an aircraft that the production rule
	// then refused: "ambiguous" (-strict) or "min_score". Absent when attached.
	Refused string `json:"refused,omitempty"`
	// Reading is which reading named the aircraft: "en" or "fr" on a database,
	// the pass on a capture.
	Reading string `json:"reading"`
	// Words are the token ranges of that reading that named the aircraft.
	Words [][2]int `json:"words,omitempty"`
}

type jsonReading struct {
	Text string `json:"text"`
	phraseology.Anatomy
}

type jsonRecord struct {
	// ID is the transcription's id on a database, "pass|frequency|file" on a
	// capture.
	ID          any    `json:"id"`
	FrequencyID string `json:"frequency_id"`
	// CreatedAt is the value as stored: created_at on a database, the capture's
	// local start time on a capture.
	CreatedAt string `json:"created_at"`
	// Covered says whether ADS-B was recorded around the transmission; Fleet is
	// the number of aircraft in its sky, after the frequency's sector.
	Covered  bool                   `json:"covered"`
	Fleet    int                    `json:"fleet"`
	Match    *jsonMatch             `json:"match"`
	Readings map[string]jsonReading `json:"readings"`
}

var (
	jsonFile *os.File
	jsonBuf  *bufio.Writer
	jsonOut  *json.Encoder
)

func openJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("-json: %w", err)
	}
	jsonFile, jsonBuf = f, bufio.NewWriter(f)
	jsonOut = json.NewEncoder(jsonBuf)
	jsonOut.SetEscapeHTML(false)
	return nil
}

func closeJSON() error {
	if jsonFile == nil {
		return nil
	}
	if err := jsonBuf.Flush(); err != nil {
		return err
	}
	return jsonFile.Close()
}

func emitJSON(r *jsonRecord) {
	if jsonOut != nil {
		_ = jsonOut.Encode(r)
	}
}

func jsonReadingOf(m *phraseology.Matcher, text string) jsonReading {
	return jsonReading{Text: text, Anatomy: m.Anatomy(phraseology.Parse(text))}
}

func jsonMatchOf(m phraseology.Match, reading, refused string) *jsonMatch {
	return &jsonMatch{Flight: m.Callsign, Hex: m.Hex, Score: m.Score, Reason: m.Reason,
		Ambiguous: m.Ambiguous, Runners: m.Runners, Refused: refused, Reading: reading, Words: m.Words}
}
