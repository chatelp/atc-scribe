package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// legacyWithBackend writes the frozen legacy configuration with the backend of
// one section ("transcription" or "post_processing") changed to line.
func legacyWithBackend(t *testing.T, section, line string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("internal", "config", "testdata", "legacy-full.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "\n["+section+"]\n")
	at := strings.Index(text[start:], "\nbackend = \"local\"\n")
	if start < 0 || at < 0 {
		t.Fatalf("fixture has no backend in [%s]", section)
	}
	at += start + 1
	text = text[:at] + line + text[at+len(`backend = "local"`):]
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The OpenAI backend is gone. A configuration that still asks for it must stop
// the server with the reason, not start it transcribing nothing.
func TestTheRemovedOpenAIBackendIsRefusedWithItsReason(t *testing.T) {
	atRepoRoot(t)
	for _, section := range []string{"transcription", "post_processing"} {
		t.Run(section, func(t *testing.T) {
			cfg, err := Load(legacyWithBackend(t, section, `backend = "openai"`))
			if err != nil {
				t.Fatal(err)
			}
			err = cfg.Validate()
			want := section + `.backend = "openai": the OpenAI backend was removed, use "local"`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("got %v, want %q", err, want)
			}
		})
	}
}

// An empty backend used to mean OpenAI. There is now one backend, so empty
// means local.
func TestAnEmptyBackendMeansLocal(t *testing.T) {
	atRepoRoot(t)
	for _, section := range []string{"transcription", "post_processing"} {
		cfg := loadAndValidate(t, legacyWithBackend(t, section, "# no backend"))
		if cfg.Transcription.Backend != BackendLocal || cfg.PostProcessing.Backend != BackendLocal {
			t.Errorf("[%s] without backend: got %q and %q, want %q", section,
				cfg.Transcription.Backend, cfg.PostProcessing.Backend, BackendLocal)
		}
	}
}

// frozenExample is configs/config.toml.example as it stood before its
// reduction (docs-fr/fiches-cloud/D-config-reduite.md), every key written out.
const frozenExample = "internal/config/testdata/example-2026-10-10.toml"

// The reduction removes from the example every key whose value is the code's
// default. Absent, such a key must give exactly the value it gave present: the
// frozen full example and the shipped one must load, validate and come out as
// the same Config, field by field.
func TestTheReducedExampleLoadsTheSameConfiguration(t *testing.T) {
	atRepoRoot(t)
	full := loadAndValidate(t, frozenExample)
	reduced := loadAndValidate(t, filepath.Join("configs", "config.toml.example"))

	// Where each was read from is not configuration.
	full.ConfigPath, reduced.ConfigPath = "", ""

	if diffs := diffValues("", reflect.ValueOf(*full), reflect.ValueOf(*reduced)); len(diffs) > 0 {
		t.Errorf("the reduced example loads differently from the full one:\n  %s",
			strings.Join(diffs, "\n  "))
	}
	if !reflect.DeepEqual(full, reduced) {
		t.Error("reflect.DeepEqual disagrees with the field walk; the walk misses something")
	}
}

// diffValues walks two values of the same type and names every leaf that
// differs, as section.field = full | reduced, so a failure says which key.
func diffValues(path string, a, b reflect.Value) []string {
	switch a.Kind() {
	case reflect.Struct:
		var out []string
		for i := 0; i < a.NumField(); i++ {
			name := a.Type().Field(i).Tag.Get("toml")
			if name == "" || name == "-" {
				name = a.Type().Field(i).Name
			}
			out = append(out, diffValues(path+"."+name, a.Field(i), b.Field(i))...)
		}
		return out
	case reflect.Slice:
		if a.Len() != b.Len() {
			return []string{fmt.Sprintf("%s: %d elements | %d", path, a.Len(), b.Len())}
		}
		var out []string
		for i := 0; i < a.Len(); i++ {
			out = append(out, diffValues(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i))...)
		}
		return out
	case reflect.Ptr:
		if a.IsNil() != b.IsNil() {
			return []string{fmt.Sprintf("%s: %v | %v", path, a, b)}
		}
		if a.IsNil() {
			return nil
		}
		return diffValues(path, a.Elem(), b.Elem())
	default:
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			return []string{fmt.Sprintf("%s: %v | %v", path, a.Interface(), b.Interface())}
		}
		return nil
	}
}
