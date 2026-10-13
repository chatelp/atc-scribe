package phraseology

import (
	"sort"
	"strings"
)

// Instruction is one thing a transmission says, in plain words: "Descend to
// FL100", "Contact 119.255", "Cleared to land, runway 27R". Role is the
// value's role, or "clearance".
type Instruction struct {
	Role string `json:"role"`
	Text string `json:"text"`
	// The same instruction in plain French, from a template per role (13/10,
	// D77): the interface stays in English, the reading follows the listener.
	TextFR string `json:"text_fr,omitempty"`
}

// verbs that give a value its instruction. A value's keyword starts at
// Value.Word; its verb is in the few words before ("descend to flight level",
// "climb and maintain") or in the keyword itself ("turn left heading").
var instructionVerbs = map[string]string{
	"descend": "descend", "descending": "descend", "descendez": "descend", "descendre": "descend", "redescendez": "descend",
	"climb": "climb", "climbing": "climb", "montez": "climb", "monter": "climb", "remontez": "climb",
	"maintain": "maintain", "maintaining": "maintain", "maintenez": "maintain",
	"passing": "passing", "leaving": "leaving",
	"left": "left", "gauche": "left", "right": "right", "droite": "right",
	"reduce": "reduce", "reduire": "reduce", "reduisez": "reduce",
	"increase": "increase", "augmentez": "increase",
	"contact": "contact", "contactez": "contact", "monitor": "contact", "call": "contact", "appelez": "contact",
}

// lookBack is how many words before a value's keyword its verb may be.
const lookBack = 3

// turnWords make a following "left" or "right" a turn ("turn left", "to the
// right", French "à droite"); after a number it is a runway side ("one two left").
var turnWords = map[string]bool{"turn": true, "turning": true, "tournez": true, "virez": true, "the": true, "a": true}

// Plain is the transmission's instructions in plain words, clearances first,
// each said once: a readback repeats the instruction it confirms.
func (r Result) Plain() []Instruction {
	toks, _ := tokenSpans(r.Raw)
	var out []Instruction
	seen := map[string]bool{}
	add := func(role, text, fr string) {
		if text == "" || seen[text] {
			return
		}
		seen[text] = true
		out = append(out, Instruction{Role: role, Text: text, TextFR: fr})
	}
	// One instruction per value: "right heading 024 … heading 024" is one.
	said := map[string]bool{}

	runways := map[string]bool{}
	for _, c := range r.Clearances {
		what := map[string]string{
			"takeoff":  "Cleared for takeoff",
			"landing":  "Cleared to land",
			"approach": "Cleared for the approach",
		}[c.Type]
		if what == "" {
			continue
		}
		fr := map[string]string{
			"takeoff":  "Autorisé à décoller",
			"landing":  "Autorisé à atterrir",
			"approach": "Autorisé à l'approche",
		}[c.Type]
		if c.Runway != "" {
			what += ", runway " + c.Runway
			fr += ", piste " + c.Runway
			runways[c.Runway] = true
		}
		add("clearance", what, fr)
	}

	values := append([]Value(nil), r.Values...)
	sort.SliceStable(values, func(i, j int) bool { return values[i].Word < values[j].Word })
	for _, v := range values {
		if said[string(v.Role)+v.Text] {
			continue
		}
		said[string(v.Role)+v.Text] = true
		verb := verbOf(toks, v)
		switch v.Role {
		case RoleFlightLevel, RoleAltitude:
			level := v.Text
			if v.Role == RoleAltitude {
				level = noLeadingZeros(v.Text) + " ft"
			}
			// "niveau 100" for a flight level, "3000 ft" for an altitude: the
			// French phraseology says "niveau", never "FL".
			levelFR := level
			if v.Role == RoleFlightLevel {
				levelFR = "niveau " + strings.TrimPrefix(level, "FL")
			}
			switch verb {
			case "descend":
				add(string(v.Role), "Descend to "+level, "Descendez au "+levelFR)
			case "climb":
				add(string(v.Role), "Climb to "+level, "Montez au "+levelFR)
			case "maintain":
				add(string(v.Role), "Maintain "+level, "Maintenez le "+levelFR)
			case "passing":
				add(string(v.Role), "Passing "+level, "En passant le "+levelFR)
			case "leaving":
				add(string(v.Role), "Leaving "+level, "En quittant le "+levelFR)
			default:
				add(string(v.Role), level, levelFR)
			}
		case RoleHeading:
			switch verb {
			case "left":
				add(string(v.Role), "Turn left heading "+v.Text, "Tournez à gauche au cap "+v.Text)
			case "right":
				add(string(v.Role), "Turn right heading "+v.Text, "Tournez à droite au cap "+v.Text)
			default:
				add(string(v.Role), "Heading "+v.Text, "Cap "+v.Text)
			}
		case RoleSpeed:
			kt := noLeadingZeros(v.Text) + " kt"
			switch verb {
			case "reduce":
				add(string(v.Role), "Reduce speed to "+kt, "Réduisez la vitesse à "+kt)
			case "increase":
				add(string(v.Role), "Increase speed to "+kt, "Augmentez la vitesse à "+kt)
			case "maintain":
				add(string(v.Role), "Maintain "+kt, "Maintenez "+kt)
			default:
				add(string(v.Role), "Speed "+kt, "Vitesse "+kt)
			}
		case RoleFrequency:
			if verb == "contact" {
				add(string(v.Role), "Contact "+v.Text, "Contactez "+v.Text)
			} else {
				add(string(v.Role), "Frequency "+v.Text, "Fréquence "+v.Text)
			}
		case RoleQNH:
			add(string(v.Role), "QNH "+v.Text, "QNH "+v.Text)
		case RoleSquawk:
			add(string(v.Role), "Squawk "+v.Text, "Transpondeur "+v.Text)
		case RoleRunway:
			if !runways[v.Text] {
				add(string(v.Role), "Runway "+v.Text, "Piste "+v.Text)
			}
		}
	}
	return out
}

// verbOf is the verb of a value: in its keyword or the few words before it,
// the nearest one. Climbing or descending wins over maintaining: "climb and
// maintain FL100" is a climb.
func verbOf(toks []string, v Value) string {
	found := ""
	end := v.End
	if end > len(toks) {
		end = len(toks)
	}
	for j := end - 1; j >= 0 && j >= v.Word-lookBack; j-- {
		v, ok := instructionVerbs[toks[j]]
		if !ok {
			continue
		}
		if (v == "left" || v == "right") && (j == 0 || !turnWords[toks[j-1]]) {
			continue
		}
		if v == "climb" || v == "descend" {
			return v
		}
		if found == "" {
			found = v
		}
	}
	return found
}

// noLeadingZeros: "01500" is said "one five hundred" and read 1500.
func noLeadingZeros(s string) string {
	t := strings.TrimLeft(s, "0")
	if t == "" {
		return s
	}
	return t
}
