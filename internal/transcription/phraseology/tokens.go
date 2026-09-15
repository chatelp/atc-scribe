// Package phraseology turns raw ATC speech-to-text into structured facts, using
// the closed vocabulary of ICAO radiotelephony rather than a language model.
//
// The design follows what measurement says about the transcription models in use:
// they hear digits reasonably and airline names badly — inventing "Air Berlin" and
// "Germanwings", carriers that no longer exist, because their fine-tuning corpora
// are years old. So every number is extracted with the role its preceding keyword
// gives it, and callsigns are matched on their digits first.
package phraseology

import "strings"

// numberWords maps spoken digits to their value. ATC speaks digits individually,
// so "three five zero" is 3-5-0 and never "three hundred and fifty" — but models
// do emit grouped forms, which groupedNumbers covers.
var numberWords = map[string]string{
	"zero": "0", "oh": "0", "o": "0", "nought": "0",
	"one": "1", "won": "1",
	"two": "2", "to": "2", "too": "2",
	"three": "3", "tree": "3",
	"four": "4", "for": "4", "fower": "4",
	"five": "5", "fife": "5",
	"six":   "6",
	"seven": "7",
	"eight": "8", "ait": "8",
	"nine": "9", "niner": "9",
}

// groupedNumbers are forms models produce even though ICAO phraseology avoids
// them: "flight level three hundred" for FL300, "one thousand" for 1000.
var groupedNumbers = map[string]int{
	"ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14,
	"fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19,
	"twenty": 20, "thirty": 30, "forty": 40, "fourty": 40, "fifty": 50,
	"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
}

const (
	wordHundred  = "hundred"
	wordThousand = "thousand"
)

// natoAlphabet maps spoken letters to their character. The spellings include the
// ones transcription models actually produce — "alfa" and "alpha", "juliett" and
// "juliet", "x-ray" arriving as "xray".
var natoAlphabet = map[string]byte{
	"alfa": 'A', "alpha": 'A',
	"bravo":   'B',
	"charlie": 'C',
	"delta":   'D',
	"echo":    'E',
	"foxtrot": 'F', "fox": 'F',
	"golf": 'G', "gulf": 'G',
	"hotel":   'H',
	"india":   'I',
	"juliett": 'J', "juliet": 'J',
	"kilo":     'K',
	"lima":     'L',
	"mike":     'M',
	"november": 'N',
	"oscar":    'O',
	"papa":     'P',
	"quebec":   'Q',
	"romeo":    'R',
	"sierra":   'S',
	"tango":    'T',
	"uniform":  'U',
	"victor":   'V',
	"whiskey":  'W', "whisky": 'W',
	"xray": 'X', "x-ray": 'X',
	"yankee": 'Y',
	"zulu":   'Z',
}

// homophones are ordinary English words that a transcript may or may not mean as
// digits. "flight level three eight zero to tepac" is FL380 followed by a
// preposition, not FL3802 — a real transcript from this station, and the reason
// these are only read as digits when another digit follows.
var homophones = map[string]bool{
	"to": true, "too": true, "for": true, "won": true, "o": true, "oh": true,
	"ait": true, "tree": true,
}

// isDigitWord reports whether w is a spoken single digit.
func isDigitWord(w string) (string, bool) {
	d, ok := numberWords[w]
	return d, ok
}

// isLetterWord reports whether w is a NATO alphabet letter.
func isLetterWord(w string) (byte, bool) {
	c, ok := natoAlphabet[w]
	return c, ok
}

// tokenize lowercases and splits on anything that is not a letter, digit or the
// hyphen inside "x-ray". Transcripts arrive with stray punctuation and casing.
func tokenize(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			cur.WriteRune(r)
		case r == '-':
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}
