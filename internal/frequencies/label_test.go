package frequencies

import "testing"

// A mixed stream's contents change while the server runs, and only whoever
// changed them knows. The configured name is the fallback, not the truth.
func TestALiveLabelOverridesTheConfiguredName(t *testing.T) {
	l := newLabels()
	if got := l.nameFor("mix", "from the config"); got != "from the config" {
		t.Errorf("with nothing set, the configured name stands: got %q", got)
	}
	if err := l.Set("mix", "CDG approaches"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := l.nameFor("mix", "from the config"); got != "CDG approaches" {
		t.Errorf("got %q, want the live label", got)
	}
}

// Clearing has to be possible, or a wrong label set once is permanent.
func TestAnEmptyLabelRestoresTheConfiguredName(t *testing.T) {
	l := newLabels()
	_ = l.Set("mix", "something")
	if err := l.Set("mix", "   "); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := l.nameFor("mix", "from the config"); got != "from the config" {
		t.Errorf("got %q, want the configured name back", got)
	}
}

// Refused rather than truncated: a label silently cut in half is worse than one
// that never arrives, because nobody looks for a message that was not shown.
func TestAnOverlongLabelIsRefused(t *testing.T) {
	l := newLabels()
	long := make([]rune, 121)
	for i := range long {
		long[i] = 'x'
	}
	if err := l.Set("mix", string(long)); err == nil {
		t.Fatal("a label past the limit should be refused")
	}
	if got := l.nameFor("mix", "from the config"); got != "from the config" {
		t.Errorf("a refused label must not be stored, got %q", got)
	}
}

func TestLabelsAreSeparatePerFrequency(t *testing.T) {
	l := newLabels()
	_ = l.Set("a", "one")
	if got := l.nameFor("b", "配置"); got != "配置" {
		t.Errorf("setting one frequency must not touch another, got %q", got)
	}
}
