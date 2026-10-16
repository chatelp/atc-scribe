package phraseology

import (
	"strings"
	"sync/atomic"
)

// The procedures heard, named (docs-fr/05-decisions.md, Q55, usage 3).
//
// The model writes "elcope seven golf" where the controller said "ELCOB 7
// Golf", a departure procedure. The page leaves the words as heard and adds a
// chip with the official name: never a correction, a proposal. It is shown
// only when it was measured right: on the owner's listening of 77 cases of
// Paris approaches (16/10), comparing the word with the points the station's
// SID and STAR are named after, a sound likeness of 0.95 or more named 18, all
// right; 0.90 named 19, 18 right; 1.10 named 9, all right. The same words were
// tried as a matching rule and withdrawn: a callsign whose airline is misheard
// ("Eziad Three Foxtrot Four") has the very shape of a procedure, so these
// chips never decide which aircraft spoke.
//
// The shape is the lab's (whisper-lab, 15/10): a word of four letters or more
// that is neither phraseology, nor a number, nor a NATO letter, nor a word of
// an airline's name; then one digit and one NATO letter; then anything but a
// NATO letter ("Anatolia Three Foxtrot Quebec" is a callsign); the word before
// not a word of an airline's name.

// PlacePoint is a point a procedure is named after.
type PlacePoint struct {
	Name     string
	Lat, Lon float64
	key      string
}

// PlaceIndex is the station's procedure points, ready to be compared by sound.
type PlaceIndex struct {
	points []PlacePoint
}

// NewPlaceIndex keys each point by its sound.
func NewPlaceIndex(points []PlacePoint) *PlaceIndex {
	ix := &PlaceIndex{}
	for _, p := range points {
		p.Name = strings.ToUpper(strings.TrimSpace(p.Name))
		if p.key = soundKey(p.Name); p.key != "" {
			ix.points = append(ix.points, p)
		}
	}
	return ix
}

// Len says how many points the index holds.
func (ix *PlaceIndex) Len() int {
	if ix == nil {
		return 0
	}
	return len(ix.points)
}

// Place is a procedure heard: where its words are and the name proposed.
type Place struct {
	Words      [2]int  // token range: the point's word, the digit and the letter
	Point      string  // ELCOB
	Designator string  // ELCOB 7G
	Lat, Lon   float64 // the point, for the map
	Score      float64 // the sound likeness, for the record
}

// Places finds the procedures heard in a reading. The matcher brings the words
// of the airlines' names; floor is the sound likeness asked for.
func (m *Matcher) Places(r Result, ix *PlaceIndex, floor float64) []Place {
	if ix.Len() == 0 {
		return nil
	}
	toks := tokenize(r.Raw)
	var out []Place
	for _, g := range alnumGroups(toks) {
		if len(g.text) != 2 || g.text[0] < '0' || g.text[0] > '9' {
			continue
		}
		i := g.word[0]
		if i < 1 {
			continue
		}
		raw := toks[i-1]
		w := fold(raw)
		if len(w) < 4 || phraseologyWords[w] || opensNumber(raw) || m.operatorWord(raw) {
			continue
		}
		if _, ok := isLetterWord(raw); ok {
			continue
		}
		// The word before may be a flight number's last digit ("Air France five
		// two, elcope seven golf"); a number is no airline's name, whatever
		// airlines.dat holds.
		if i >= 2 && m.operatorWord(toks[i-2]) && !opensNumber(toks[i-2]) {
			continue
		}
		best, score := ix.nearest(raw)
		if best == nil || score < floor {
			continue
		}
		out = append(out, Place{
			Words: [2]int{i - 1, g.word[1]}, Point: best.Name, Designator: best.Name + " " + g.text,
			Lat: best.Lat, Lon: best.Lon, Score: score,
		})
	}
	return out
}

func (ix *PlaceIndex) nearest(word string) (*PlacePoint, float64) {
	k := soundKey(word)
	var best *PlacePoint
	bestScore := 0.0
	for i := range ix.points {
		if s := soundScore(k, ix.points[i].key); s > bestScore {
			best, bestScore = &ix.points[i], s
		}
	}
	return best, bestScore
}

// PlaceNamer holds the index in force, shared by the API and the processor,
// so that a list made again from the settings page applies to the next read.
type PlaceNamer struct {
	matcher *Matcher
	index   atomic.Pointer[PlaceIndex]
}

// NewPlaceNamer names with this matcher's airline words.
func NewPlaceNamer(m *Matcher) *PlaceNamer { return &PlaceNamer{matcher: m} }

// Set gives the namer the station's procedure points; nil takes them away.
func (n *PlaceNamer) Set(points []PlacePoint) {
	n.index.Store(NewPlaceIndex(points))
}

// Count says how many procedure points are in force.
func (n *PlaceNamer) Count() int {
	if n == nil {
		return 0
	}
	return n.index.Load().Len()
}

// Name finds the procedures heard in a text, at the sound likeness asked for.
func (n *PlaceNamer) Name(r Result, floor float64) []Place {
	if n == nil || n.matcher == nil {
		return nil
	}
	return n.matcher.Places(r, n.index.Load(), floor)
}

var accents = strings.NewReplacer("à", "a", "â", "a", "ä", "a", "á", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "í", "i", "ô", "o", "ö", "o", "ó", "o", "ù", "u", "û", "u", "ü", "u", "ú", "u",
	"ç", "c", "ÿ", "y", "ñ", "n", "œ", "oe", "æ", "ae")

// fold lowercases and drops accents and anything that is not a letter.
func fold(w string) string {
	var b strings.Builder
	for _, r := range accents.Replace(strings.ToLower(w)) {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// addOperatorWords records the words of an airline's name or telephony.
func (m *Matcher) addOperatorWords(form string) {
	if m.operatorWords == nil {
		return
	}
	for _, w := range strings.FieldsFunc(strings.ToLower(form), func(r rune) bool { return r < 'a' || r > 'z' }) {
		if len(w) >= 2 {
			m.operatorWords[w] = true
		}
	}
}

func (m *Matcher) operatorWord(w string) bool {
	return m.operatorWords != nil && m.operatorWords[fold(w)]
}

// soundKey spells a word the way it sounds, crudely: what the model's spelling
// and the official name share ("elcope", "Elkhob" and ELCOB). The lab's rules
// (15/10): ph f, qu ck c q k, x ks, y i, w v, z s, h dropped; b p, d t and g k
// alike; doubled letters once.
func soundKey(w string) string {
	s := fold(w)
	for _, r := range [][2]string{{"ph", "f"}, {"qu", "k"}, {"ck", "k"}, {"x", "ks"}, {"c", "k"}, {"q", "k"},
		{"y", "i"}, {"w", "v"}, {"z", "s"}, {"h", ""}} {
		s = strings.ReplaceAll(s, r[0], r[1])
	}
	s = strings.NewReplacer("b", "p", "d", "t", "g", "k").Replace(s)
	var b strings.Builder
	var last rune
	for _, r := range s {
		if r != last {
			b.WriteRune(r)
		}
		last = r
	}
	return b.String()
}

// soundScore scores two sound keys: 2·LCS over their lengths, 0.15 more when
// they start alike (the lab's score).
func soundScore(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			switch {
			case a[i-1] == b[j-1]:
				cur[j] = prev[j-1] + 1
			case prev[j] > cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	s := 2 * float64(prev[len(b)]) / float64(len(a)+len(b))
	if a[0] == b[0] {
		s += 0.15
	}
	return s
}

// phraseologyWords are not points' names: the lab's lists of English and French
// phraseology (whisper-lab/scripts/labo/juges.py, charabia-premier.py).
var phraseologyWords = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(`
good morning afternoon evening day hello bye goodbye roger wilco affirm affirmative negative correct
thanks thank you please say again standby stand by descend descending climb climbing maintain maintaining
reduce reducing increase increasing speed knots turn turning left right heading direct proceed proceeding
cleared clear approach ils localizer loc glide slope established intercept intercepting runway land landing
takeoff take off contact frequency monitor tower ground radar identified squawk ident level flight altitude
feet qnh information report expect vectors via to for on the and with at of in until after when ready request
requesting traffic final miles degrees now we are is your our sir madam de gaulle paris orly charles bonjour
merci au revoir number hundred thousand decimal point
minimum maximum no restriction restrictions free own navigation continue join resume due behind follow
airborne passing leaving reaching visual caution wind check go around missed stop stopping hold holding
confirm copy copied that this it be will can could arrival departure sector control centre center
x-ray okay ok up down time call from reported reporting minutes back set over route as not more depart departing
airport star unable able all so then just still same also next last first second third again shortly short
bonsoir monsieur madame bonne journee soiree autorise autorisee autorises atterrissage decollage piste pistes
approches niveau niveaux descendez descends descendons descente montez monte montons montee maintenez maintiens
maintenons cap caps virez virage gauche droite directement vers rejoignez rejoins contactez contacte contacter
rappelez rappelle rappel frequence frequences vitesse reduisez reduis reduisons noeuds kts pieds transpondeur
affichez affiche identifie identifies controle tour sol prevol departs arrivee arrivees roulez roule roulage
attente alignez aligne alignement remettez remise gaz interception interceptez intercepte trafic informations
vent calme variable degres dans le la les du des et au aux en sur sous pour par avec sans plus moins ou est
sommes suis vous nous je on il elle ce cette votre vos que qui pas bon oui non droit tout voici voila alors
tournez tourne bas haut euh etes avez avons fait faites prochaine prochain deja encore`) {
		out[w] = true
	}
	return out
}()
