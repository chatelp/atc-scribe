package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
}

// Runtime holds the live values and persists changes.
type Runtime struct {
	mu       sync.RWMutex
	settings RuntimeSettings
	path     string
	log      *logger.Logger
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
			DBRetentionDays: cfg.Storage.DBRetentionDays,
			LogLevel:        cfg.Logging.Level,
		},
		path: filepath.Join(filepath.Dir(configPath), RuntimeSettingsFile),
		log:  log.Named("runtime-settings"),
	}

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
	r.log.Info("Applied saved runtime settings",
		logger.String("path", r.path),
		logger.Int("db_retention_days", r.settings.DBRetentionDays),
		logger.String("log_level", r.settings.LogLevel))
	return r
}

// Settings returns a copy of the values in force.
func (r *Runtime) Settings() RuntimeSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.settings
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

	if err := logger.SetLevel(next.LogLevel); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

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
		logger.String("log_level", next.LogLevel))
	return nil
}
