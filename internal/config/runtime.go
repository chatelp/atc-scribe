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

	// AlsoAirports are followed besides the reference airport: each aircraft is
	// judged against the one whose runway axis it flies, else the nearest. Orly
	// and De Gaulle are 13 NM apart and a receiver near Paris sees both
	// (docs-fr/05-decisions.md, Q40). The weather stays the reference airport's.
	AlsoAirports []string `json:"also_airports,omitempty"`

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

	// An aircraft heard on the frequency in the last two minutes may be named
	// by an abbreviation: its last letters ("sierra bravo"), its last two
	// digits when no other aircraft just heard ends so, or its airline alone
	// when it is the only one of that airline just heard. Measured on 24/09
	// against another frequency's recent aircraft: +8 and +13 true matches,
	// almost no chance; the airline alone +68, but ADS-B cannot tell whether
	// it named the right aircraft, so it stays off until checked by ear.
	ContextLetters bool `json:"context_letters"`
	ContextDigits  bool `json:"context_digits"`
	ContextNames   bool `json:"context_names"`

	// Sectors compares a transmission only with the aircraft its frequency can
	// be talking to, for the frequencies that name an airport and a kind. The
	// radii and ceilings were agreed with the owner on 26/09. Measured on 24/09
	// on De Gaulle approach, then on 26/09 on Orly and three De Gaulle
	// frequencies: chance divided by 2.4, as many right aircraft, precision
	// 91% -> 96% (docs-fr/05-decisions.md, D59).
	Sectors bool                  `json:"sectors"`
	Sector  map[string]SectorSize `json:"sector"`
}

// SectorSize is how far and how high a kind of frequency reaches.
type SectorSize struct {
	RadiusNM float64 `json:"radius_nm"`
	MaxAltFt float64 `json:"max_alt_ft"`
}

// DefaultSectorSizes are the radii and ceilings agreed on 26/09.
func DefaultSectorSizes() map[string]SectorSize {
	return map[string]SectorSize{
		"approach":  {RadiusNM: 60, MaxAltFt: 20000},
		"departure": {RadiusNM: 60, MaxAltFt: 20000},
		"tower":     {RadiusNM: 15, MaxAltFt: 6000},
		"ground":    {RadiusNM: 5, MaxAltFt: 1500},
	}
}

// DefaultMatchingRules are the rules in force until changed from the panel:
// letters, approximate names, and an aircraft just heard named by its last
// letters or digits, decided by the owner on 26/09 after Q46; and each
// frequency's sector, once validated on 26/09 (D59).
func DefaultMatchingRules(minDigits int) MatchingRules {
	if minDigits <= 0 {
		minDigits = 3
	}
	return MatchingRules{Letters: true, ApproxOperators: true, MinDigits: minDigits,
		ContextLetters: true, ContextDigits: true, Sectors: true, Sector: DefaultSectorSizes()}
}

func (m MatchingRules) validate() error {
	if m.MinDigits < 2 || m.MinDigits > 4 {
		return fmt.Errorf("matching.min_digits must be 2, 3 or 4, got %d", m.MinDigits)
	}
	for kind, size := range m.Sector {
		if !SectorKinds[kind] {
			return fmt.Errorf("matching.sector: unknown kind %q", kind)
		}
		if size.RadiusNM < 1 || size.RadiusNM > 250 {
			return fmt.Errorf("matching.sector.%s.radius_nm must be between 1 and 250, got %g", kind, size.RadiusNM)
		}
		if size.MaxAltFt < 500 || size.MaxAltFt > 60000 {
			return fmt.Errorf("matching.sector.%s.max_alt_ft must be between 500 and 60000, got %g", kind, size.MaxAltFt)
		}
	}
	return nil
}

// withSectorDefaults fills the kinds a saved file does not mention, so that a
// kind added later has a size without anyone having to set it.
func (m *MatchingRules) withSectorDefaults() {
	if m.Sector == nil {
		m.Sector = map[string]SectorSize{}
	}
	for kind, size := range DefaultSectorSizes() {
		if _, ok := m.Sector[kind]; !ok {
			m.Sector[kind] = size
		}
	}
}

// MaxAlsoAirports bounds the airports followed besides the reference one. Two
// or three is what one receiver can watch; each costs a runway tracker, and the
// measurement of 23/09 found little beyond Orly and De Gaulle.
const MaxAlsoAirports = 3

// ReferenceAirports returns the airports followed, the reference one first.
func (s RuntimeSettings) ReferenceAirports() []string {
	if s.ReferenceAirport == "" {
		return nil
	}
	return append([]string{s.ReferenceAirport}, s.AlsoAirports...)
}

// normalizeAlso uppercases the further airports and drops blanks, repeats and
// the reference airport itself.
func normalizeAlso(principal string, codes []string) []string {
	var out []string
	seen := map[string]bool{principal: true}
	for _, c := range codes {
		c = normalizeAirport(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func sameAirports(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ReferenceAirportHook validates and applies a change of reference airports,
// the reference one first. The config package cannot do either itself -- it
// takes the reference data, the ADS-B service and the weather service -- so the
// server wires them in.
type ReferenceAirportHook struct {
	Validate func(codes []string) error // no side effect: refuse before anything moves
	Apply    func(codes []string)       // cannot fail once Validate has passed
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
		also := normalizeAlso(a, saved.AlsoAirports)
		if len(also) > MaxAlsoAirports {
			also = also[:MaxAlsoAirports]
		}
		r.settings.AlsoAirports = also
	}
	if saved.Matching != nil {
		saved.Matching.withSectorDefaults()
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
		logger.String("reference_airport", r.settings.ReferenceAirport),
		logger.String("also_airports", strings.Join(r.settings.AlsoAirports, ",")))
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

// UseReferenceAirports records the airports actually in force at startup, the
// reference one first, when some saved from the panel turned out unusable and
// the server fell back or left them out. It is not written to disk: the saved
// choice stays, and each start says in the log why it was not used.
func (r *Runtime) UseReferenceAirports(codes []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(codes) == 0 {
		return
	}
	r.settings.ReferenceAirport = normalizeAirport(codes[0])
	r.settings.AlsoAirports = normalizeAlso(r.settings.ReferenceAirport, codes[1:])
}

// Settings returns a copy of the values in force.
func (r *Runtime) Settings() RuntimeSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := r.settings
	if s.Matching != nil {
		m := s.Matching.clone()
		s.Matching = &m
	}
	return s
}

// clone copies the rules, sizes included: a caller that edits its copy must
// not change the rules in force.
func (m MatchingRules) clone() MatchingRules {
	c := m
	c.Sector = make(map[string]SectorSize, len(m.Sector))
	for k, v := range m.Sector {
		c.Sector[k] = v
	}
	return c
}

// Matching returns the association rules in force. It is read for every
// transmission, so a change from the panel applies to the next one.
func (r *Runtime) Matching() MatchingRules {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.settings.Matching.clone()
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
	next.Matching.withSectorDefaults()
	if err := next.Matching.validate(); err != nil {
		return err
	}

	// The reference airports are checked with the rest, before anything is applied.
	next.ReferenceAirport = normalizeAirport(next.ReferenceAirport)
	next.AlsoAirports = normalizeAlso(next.ReferenceAirport, next.AlsoAirports)
	if len(next.AlsoAirports) > MaxAlsoAirports {
		return fmt.Errorf("also_airports: %d at most, got %d", MaxAlsoAirports, len(next.AlsoAirports))
	}
	r.mu.RLock()
	current, hook := r.settings.ReferenceAirports(), r.airportHook
	r.mu.RUnlock()
	airportChanged := !sameAirports(next.ReferenceAirports(), current)
	if airportChanged {
		if next.ReferenceAirport == "" {
			return fmt.Errorf("reference_airport cannot be empty")
		}
		if hook == nil {
			return fmt.Errorf("reference_airport cannot be changed while the server runs in this configuration")
		}
		if err := hook.Validate(next.ReferenceAirports()); err != nil {
			return fmt.Errorf("reference_airport: %w", err)
		}
	}

	if err := logger.SetLevel(next.LogLevel); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}
	if airportChanged {
		hook.Apply(next.ReferenceAirports())
	}

	m := next.Matching.clone()
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
		logger.String("also_airports", strings.Join(next.AlsoAirports, ",")),
		logger.Bool("match_letters", m.Letters),
		logger.Bool("match_approx_operators", m.ApproxOperators),
		logger.Int("match_min_digits", m.MinDigits),
		logger.Bool("match_one_digit_off", m.OneDigitOff),
		logger.Bool("match_context_letters", m.ContextLetters),
		logger.Bool("match_context_digits", m.ContextDigits),
		logger.Bool("match_context_names", m.ContextNames),
		logger.Bool("match_sectors", m.Sectors))
	return nil
}
