package adsb

import (
	"strings"
	"sync"
	"time"
)

// VoiceData is what the radio has said about an aircraft, carried on the aircraft
// itself so a client can tell, at a glance, which targets the controller has
// actually been talking to.
//
// Filtering happens in the browser in this application -- see the note on
// handleFilterUpdate -- so the server's job is to carry the fact, not to select
// on it.
type VoiceData struct {
	Transmissions int       `json:"transmissions"`
	LastHeard     time.Time `json:"last_heard"`
	LastText      string    `json:"last_text,omitempty"`
}

// VoiceIndex answers what voice has attached to a callsign. Keeping it an
// interface leaves the ADS-B service unaware of SQLite and of the transcription
// package, which is what lets either side change without the other.
type VoiceIndex interface {
	VoiceFor(callsign string) *VoiceData
}

// SetVoiceIndex attaches the transcription side. Without it every aircraft simply
// carries no voice field, which is the correct behaviour for an installation that
// transcribes nothing.
func (s *Service) SetVoiceIndex(v VoiceIndex) {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()
	s.voice = v
}

// attachVoice fills in the voice field of every aircraft that has one.
func (s *Service) attachVoice(aircraft []*Aircraft) {
	s.voiceMu.RLock()
	idx := s.voice
	s.voiceMu.RUnlock()
	if idx == nil {
		return
	}
	for _, a := range aircraft {
		callsign := strings.TrimSpace(a.Flight)
		if callsign == "" {
			continue
		}
		a.Voice = idx.VoiceFor(callsign)
	}
}

// voiceState is embedded in Service.
type voiceState struct {
	voiceMu sync.RWMutex
	voice   VoiceIndex
}
