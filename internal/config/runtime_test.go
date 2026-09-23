package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yegors/co-atc/pkg/logger"
)

func newRuntimeIn(t *testing.T, dir string) *Runtime {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	cfg.Storage.DBRetentionDays = 7
	cfg.Logging.Level = "info"
	cfg.Station.AirportCode = "LFPZ"
	return NewRuntime(cfg, filepath.Join(dir, "config.toml"), log)
}

// recorder is a hook that accepts LFPO and LFPG and remembers what it applied.
type recorder struct{ applied []string }

func (r *recorder) hook() ReferenceAirportHook {
	return ReferenceAirportHook{
		Validate: func(code string) error {
			if code == "LFPO" || code == "LFPG" {
				return nil
			}
			return errors.New("unknown airport " + code)
		},
		Apply: func(code string) { r.applied = append(r.applied, code) },
	}
}

func TestReferenceAirportDefaultsToTheConfig(t *testing.T) {
	r := newRuntimeIn(t, t.TempDir())
	if got := r.Settings().ReferenceAirport; got != "LFPZ" {
		t.Errorf("should start from [station] airport_code, got %q", got)
	}
}

func TestChangingTheAirportIsAppliedAndPersisted(t *testing.T) {
	dir := t.TempDir()
	r := newRuntimeIn(t, dir)
	rec := &recorder{}
	r.SetReferenceAirportHook(rec.hook())

	next := r.Settings()
	next.ReferenceAirport = " lfpo " // as typed
	if err := r.Apply(next); err != nil {
		t.Fatalf("a valid airport should be accepted: %v", err)
	}
	if len(rec.applied) != 1 || rec.applied[0] != "LFPO" {
		t.Errorf("the hook should have applied LFPO once, got %v", rec.applied)
	}
	if got := r.Settings().ReferenceAirport; got != "LFPO" {
		t.Errorf("setting should read LFPO, got %q", got)
	}
	// And it survives a restart.
	if got := newRuntimeIn(t, dir).Settings().ReferenceAirport; got != "LFPO" {
		t.Errorf("after a restart the saved airport should win, got %q", got)
	}
}

func TestARefusedAirportLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	r := newRuntimeIn(t, dir)
	rec := &recorder{}
	r.SetReferenceAirportHook(rec.hook())

	next := r.Settings()
	next.ReferenceAirport = "ZZZZ"
	if err := r.Apply(next); err == nil {
		t.Fatal("an unknown airport must be refused")
	}
	if len(rec.applied) != 0 {
		t.Errorf("nothing should have been applied, got %v", rec.applied)
	}
	if got := r.Settings().ReferenceAirport; got != "LFPZ" {
		t.Errorf("the airport in force should be unchanged, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, RuntimeSettingsFile)); !os.IsNotExist(err) {
		t.Errorf("a refused change must not be written to disk")
	}
}

// Validation is all or nothing: a bad retention blocks a good airport too.
func TestAnInvalidFieldBlocksTheAirportToo(t *testing.T) {
	r := newRuntimeIn(t, t.TempDir())
	rec := &recorder{}
	r.SetReferenceAirportHook(rec.hook())

	next := r.Settings()
	next.ReferenceAirport = "LFPO"
	next.DBRetentionDays = 0
	if err := r.Apply(next); err == nil {
		t.Fatal("a retention of 0 must be refused")
	}
	if len(rec.applied) != 0 {
		t.Errorf("the airport must not be applied when another field is refused, got %v", rec.applied)
	}
}

func TestAnUnchangedAirportDoesNotCallTheHook(t *testing.T) {
	r := newRuntimeIn(t, t.TempDir())
	rec := &recorder{}
	r.SetReferenceAirportHook(rec.hook())

	next := r.Settings() // LFPZ, which the hook would refuse
	next.LogLevel = "warn"
	if err := r.Apply(next); err != nil {
		t.Fatalf("changing only the log level must not re-validate the airport: %v", err)
	}
	if len(rec.applied) != 0 {
		t.Errorf("the hook should not run for an unchanged airport, got %v", rec.applied)
	}
}

// Without the hook the change cannot be applied, so it must not be accepted:
// a setting saved and never applied is what this file refuses.
func TestWithoutAHookTheAirportCannotChange(t *testing.T) {
	r := newRuntimeIn(t, t.TempDir())
	next := r.Settings()
	next.ReferenceAirport = "LFPO"
	if err := r.Apply(next); err == nil {
		t.Error("without a hook, changing the airport must be refused")
	}
}
