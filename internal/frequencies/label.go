package frequencies

import (
	"fmt"
	"strings"
	"sync"
)

// A live label for a frequency, set from outside.
//
// Upstream models a frequency entry as one radio channel with one name and one
// number in MHz, which is true of a channel and false of a mix. RTLSDR-Airband's
// mixer puts several channels on one mount, and what that mount carries can
// change while the server runs -- on the station this was written for, it does,
// because the listening group is switched there rather than here.
//
// co-atc cannot know when that happens, and it should not try: a station's group
// switcher is the station's business, and building knowledge of one installation
// into a tool meant to be published would be exactly the wrong trade. So the
// mechanism is generic and the policy stays outside -- whoever changes what is
// being broadcast says so, through the API, with whatever they already use.
//
// An empty label means "use the one from the configuration", so clearing it is
// a matter of sending an empty string.
type labels struct {
	mu sync.RWMutex
	by map[string]string
}

func newLabels() *labels { return &labels{by: map[string]string{}} }

// Set records a live label, or clears it when text is empty. It refuses a label
// long enough to break the display rather than truncating silently.
func (l *labels) Set(id, text string) error {
	text = strings.TrimSpace(text)
	if len([]rune(text)) > 120 {
		return fmt.Errorf("label is %d characters, maximum is 120", len([]rune(text)))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if text == "" {
		delete(l.by, id)
		return nil
	}
	l.by[id] = text
	return nil
}

// nameFor returns the live label if one has been set, else the configured name.
func (l *labels) nameFor(id, configured string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if live, ok := l.by[id]; ok {
		return live
	}
	return configured
}

// SetLabel records what a frequency is currently carrying, overriding the name
// from the configuration until it is cleared with an empty string.
func (s *Service) SetLabel(id, text string) error {
	if _, ok := s.frequenciesConfig[id]; !ok {
		return fmt.Errorf("no frequency with id %q", id)
	}
	return s.labels.Set(id, text)
}
