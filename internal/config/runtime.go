package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yegors/co-atc/pkg/logger"
)

// RuntimeSettings are the few operational values that can be changed while the
// server runs, from the settings panel.
//
// They live in a file of their own rather than being written back into the TOML.
// The TOML is the documentation: it carries the measured tables and the reasons
// behind every default, and a program that rewrites it would destroy exactly what
// makes it worth reading. This file records only what was changed from the panel,
// and the TOML stays the source of the defaults.
type RuntimeSettings struct {
	DBRetentionDays int    `json:"db_retention_days"`
	LogLevel        string `json:"log_level"`

	// ReferenceAirport is the airport that approach, departure, takeoff and
	// landing are judged against, and whose weather is fetched. It starts as
	// [station] airport_code and can be changed from the panel -- the airport a
	// receiver sits on is not always the one worth watching.
	ReferenceAirport string `json:"reference_airport"`

	// Matching is how a transmission is tied to an aircraft. Absent from a file
	// saved before it existed, which then means the defaults.
	Matching *MatchingRules `json:"matching,omitempty"`
}

// MatchingRules are the association rules that can be changed from the panel.
// Each was measured against a shuffled sky before being offered
// (docs-fr/05-decisions.md, Q30 and Q46).
type MatchingRules struct {
	// Letters reads a spoken number followed by spelled letters as one flight
	// part, "seven uniform echo" as 7UE: 59% of the callsigns over Paris have
	// letters. Measured on 24/09: +27% true matches, precision 73% -> 76%.
	Letters bool `json:"letters"`
	// ApproxOperators accepts an airline name heard roughly ("welling" for
	// Vueling), among the operators in the sky only.
	ApproxOperators bool `json:"approx_operators"`
	// MinDigits is the shortest spoken number that may stand for a flight
	// number: 2 matched by chance about as often as by truth, 3 is the default.
	MinDigits int `json:"min_digits"`
	// OneDigitOff accepts a number one digit away from an aircraft's. Measured
	// 81% noise; offered to try, off by default.
	OneDigitOff bool `json:"one_digit_off"`
}

// DefaultMatchingRules are the rules in force until changed from the panel:
// letters and approximate names on, decided by the owner on 26/09 after Q46.
func DefaultMatchingRules(minDigits int) MatchingRules {
	if minDigits <= 0 {
		minDigits = 3
	}
	return MatchingRules{Letters: true, ApproxOperators: true, MinDigits: minDigits}
}

func (m MatchingRules) validate() error {
	if m.MinDigits < 2 || m.MinDigits > 4 {
		return fmt.Errorf("matching.min_digits must be 2, 3 or 4, got %d", m.MinDigits)
	}
	return nil
}

// ReferenceAirportHook validates and applies a change of reference airport. The
// config package cannot do either itself -- it takes the reference data, the
// ADS-B service and the weather service -- so the server wires them in.
type ReferenceAirportHook struct {
	Validate func(code string) error // no side effect: refuse before anything moves
	Apply    func(code string)        // cannot fail once Validate has passed
}

// Runtime holds the live values and persists changes.
type Runtime struct {
	mu          sync.RWMutex
	settings    RuntimeSettings
	path        string
	log         *logger.Logger
	airportHook *ReferenceAirportHook
}

// RuntimeSettingsFile is where changes made from the panel are kept, beside the
// configuration they override.
const RuntimeSettingsFile = "runtime-settings.json"

// NewRuntime starts from the configured defaults, then applies whatever a previous
// session saved. A missing or unreadable file is not an error: it means nothing has
// been changed from the panel yet, which is the normal state.
func NewRuntime(cfg *Config, configPath string, log *logger.Logger) *Runtime {
	r := &Runtime{
		settings: RuntimeSettings{
			DBRetentionDays:  cfg.Storage.DBRetentionDays,
			LogLevel:         cfg.Logging.Level,
			ReferenceAirport: normalizeAirport(cfg.Station.AirportCode),
		},
		path: filepath.Join(filepath.Dir(configPath), RuntimeSettingsFile),
		log:  log.Named("runtime-settings"),
	}
	defaults := DefaultMatchingRules(cfg.PostProcessing.MinDigits)
	r.settings.Matching = &defaults

	b, err := os.ReadFile(r.path)
	if err != nil {
		return r
	}
	var saved RuntimeSettings
	if err := json.Unmarshal(b, &saved); err != nil {
		r.log.Warn("Ignoring unreadable runtime settings", logger.String("path", r.path), logger.Error(err))
		return r
	}
	if saved.DBRetentionDays > 0 {
		r.settings.DBRetentionDays = saved.DBRetentionDays
	}
	if saved.LogLevel != "" {
		r.settings.LogLevel = saved.LogLevel
	}
	if a := normalizeAirport(saved.ReferenceAirport); a != "" {
		r.settings.ReferenceAirport = a
	}
	if saved.Matching != nil {
		if err := saved.Matching.validate(); err != nil {
			r.log.Warn("Ignoring saved matching rules", logger.Error(err))
		} else {
			m := *saved.Matching
			r.settings.Matching = &m
		}
	}
	r.log.Info("Applied saved runtime settings",
		logger.String("path", r.path),
		logger.Int("db_retention_days", r.settings.DBRetentionDays),
		logger.String("log_level", r.settings.LogLevel),
		logger.String("reference_airport", r.settings.ReferenceAirport))
	return r
}

func normalizeAirport(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// SetReferenceAirportHook wires in how a reference airport is checked and
// applied. Without it the reference airport cannot be changed from the panel:
// a setting that is saved but never applied is the failure this file refuses.
func (r *Runtime) SetReferenceAirportHook(h ReferenceAirportHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.airportHook = &h
}

// UseReferenceAirport records the airport actually in force at startup, when the
// one saved from the panel turned out unusable and the server fell back. It is
// not written to disk: the saved choice stays, and each start says in the log
// why it was not used.
func (r *Runtime) UseReferenceAirport(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings.ReferenceAirport = normalizeAirport(code)
}

// Settings returns a copy of the values in force.
func (r *Runtime) Settings() RuntimeSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := r.settings
	if s.Matching != nil {
		m := *s.Matching
		s.Matching = &m
	}
	return s
}

// Matching returns the association rules in force. It is read for every
// transmission, so a change from the panel applies to the next one.
func (r *Runtime) Matching() MatchingRules {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return *r.settings.Matching
}

// DBRetentionDays is read on every retention tick rather than captured once, so a
// change from the panel takes effect on the next pass.
func (r *Runtime) DBRetentionDays() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.settings.DBRetentionDays
}

// Apply validates and applies a change, then persists it. Nothing is applied unless
// every field validates: a half-applied settings change is harder to reason about
// than a rejected one.
func (r *Runtime) Apply(next RuntimeSettings) error {
	if next.DBRetentionDays < 1 || next.DBRetentionDays > 365 {
		return fmt.Errorf("db_retention_days must be between 1 and 365, got %d", next.DBRetentionDays)
	}
	switch next.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log_level must be debug, info, warn or error, got %q", next.LogLevel)
	}

	if next.Matching == nil {
		return fmt.Errorf("matching rules missing")
	}
	if err := next.Matching.validate(); err != nil {
		return err
	}

	// The reference airport is checked with the rest, before anything is applied.
	next.ReferenceAirport = normalizeAirport(next.ReferenceAirport)
	r.mu.RLock()
	current, hook := r.settings.ReferenceAirport, r.airportHook
	r.mu.RUnlock()
	airportChanged := next.ReferenceAirport != current
	if airportChanged {
		if next.ReferenceAirport == "" {
			return fmt.Errorf("reference_airport cannot be empty")
		}
		if hook == nil {
			return fmt.Errorf("reference_airport cannot be changed while the server runs in this configuration")
		}
		if err := hook.Validate(next.ReferenceAirport); err != nil {
			return fmt.Errorf("reference_airport: %w", err)
		}
	}

	if err := logger.SetLevel(next.LogLevel); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}
	if airportChanged {
		hook.Apply(next.ReferenceAirport)
	}

	m := *next.Matching
	next.Matching = &m
	r.mu.Lock()
	r.settings = next
	r.mu.Unlock()

	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	// Written through a temporary file: a truncated settings file would be silently
	// ignored at the next start, and the change would vanish without a word.
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("failed to write runtime settings: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("failed to install runtime settings: %w", err)
	}

	r.log.Info("Runtime settings changed",
		logger.Int("db_retention_days", next.DBRetentionDays),
		logger.String("log_level", next.LogLevel),
		logger.String("reference_airport", next.ReferenceAirport),
		logger.Bool("match_letters", m.Letters),
		logger.Bool("match_approx_operators", m.ApproxOperators),
		logger.Int("match_min_digits", m.MinDigits),
		logger.Bool("match_one_digit_off", m.OneDigitOff))
	return nil
}
