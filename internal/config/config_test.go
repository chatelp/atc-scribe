package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// atRepoRoot runs the test from the repository root, where Validate looks for
// www/ and where the configuration's relative paths point.
func atRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
}

// loadAndValidate is what the server does at startup before anything else.
func loadAndValidate(t *testing.T, path string) *Config {
	t.Helper()
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate %s: %v", path, err)
	}
	return cfg
}

// The owner's configuration still carries [atc_chat], the OpenAI keys of
// [transcription] and those of [post_processing]. Removing the code behind them
// must not stop the server from starting with that file as it is: a key that no
// longer does anything is ignored, never refused.
func TestAConfigurationWrittenForTheRemovedPathsStillStarts(t *testing.T) {
	atRepoRoot(t)
	path := filepath.Join("internal", "config", "testdata", "legacy-full.toml")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture is only worth something while it carries the old sections.
	for _, key := range []string{"[atc_chat]", "realtime_model", "openai_api_key", `model = "gpt-4o-transcribe"`,
		"noise_reduction", "prompt_path", "turn_detection_type", "retry_max_attempts",
		`model = "gpt-4o"`, "context_transcriptions", "system_prompt_path"} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("the fixture lost %q; it must keep every legacy key", key)
		}
	}

	cfg := loadAndValidate(t, path)

	// What the local path reads is still read.
	if cfg.Transcription.Backend != "local" || cfg.PostProcessing.Backend != "local" {
		t.Errorf("backends: transcription %q, post_processing %q, want local and local",
			cfg.Transcription.Backend, cfg.PostProcessing.Backend)
	}
	if cfg.Transcription.Local.ServerURL == "" || cfg.Transcription.Local.SegmentSilenceMs == 0 {
		t.Errorf("[transcription.local] was not read: %+v", cfg.Transcription.Local)
	}
	if cfg.PostProcessing.AirlinesDatPath == "" || cfg.PostProcessing.MinDigits == 0 {
		t.Errorf("[post_processing] local keys were not read: %+v", cfg.PostProcessing)
	}
	if len(cfg.Frequencies.Sources) == 0 || cfg.ADSB.SourceType == "" {
		t.Error("frequencies or adsb were not read")
	}
}

// The example shipped with the repository must start as it is.
func TestTheExampleConfigurationStarts(t *testing.T) {
	atRepoRoot(t)
	loadAndValidate(t, filepath.Join("configs", "config.toml.example"))
}
