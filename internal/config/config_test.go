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

	// Its live keys are those of the example of the same day, so the defaults
	// the code grew since must leave it loading as that example does.
	full := loadAndValidate(t, frozenExample)
	cfg.ConfigPath, full.ConfigPath = "", ""
	if diffs := diffValues("", reflect.ValueOf(*full), reflect.ValueOf(*cfg)); len(diffs) > 0 {
		t.Errorf("the legacy configuration loads differently from the full example:\n  %s",
			strings.Join(diffs, "\n  "))
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

// writeConfig puts text in a temporary config.toml and returns its path.
func writeConfig(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// minimalConfig says only what no default can guess: where the aircraft come
// from, where the station is, and which weather to fetch.
const minimalConfig = `
[adsb]
source_type = "tar1090"
tar1090_base_url = "http://receiver.local/tar1090/data/"

[station]
latitude = 48.8
longitude = 2.1
airport_code = "LFPO"

[wx]
fetch_metar = true
`

// A key left out holds the code's default, for every section: the almost
// empty configuration above validates and comes out as Defaults plus what it
// said.
func TestAnAlmostEmptyConfigurationStartsOnTheDefaults(t *testing.T) {
	atRepoRoot(t)
	cfg := loadAndValidate(t, writeConfig(t, minimalConfig))

	want := Defaults()
	want.ADSB.SourceType, want.ADSB.Tar1090BaseURL = "tar1090", "http://receiver.local/tar1090/data/"
	want.Station.Latitude, want.Station.Longitude, want.Station.AirportCode = 48.8, 2.1, "LFPO"
	want.Weather.FetchMETAR = true
	cfg.ConfigPath = ""
	if diffs := diffValues("", reflect.ValueOf(want), reflect.ValueOf(*cfg)); len(diffs) > 0 {
		t.Errorf("not the defaults (want | got):\n  %s", strings.Join(diffs, "\n  "))
	}
	for _, check := range []struct {
		name string
		got  any
		want any
	}{
		{"cruise_altitude_ft", cfg.FlightPhases.CruiseAltitudeFt, 18000},
		{"flying_min_alt_ft", cfg.FlightPhases.FlyingMinAltFt, 700.0},
		{"airlines_dat_path", cfg.Reference.AirlinesDATPath, "assets/airlines.dat"},
		{"server_url", cfg.Transcription.Local.ServerURL, "http://127.0.0.1:8178"},
		{"opensky_base_url", cfg.ADSB.OpenSkyBaseURL, "https://opensky-network.org/api"},
		{"db_retention_gb", cfg.Storage.DBRetentionGB, DefaultDBRetentionGB},
	} {
		if !reflect.DeepEqual(check.got, check.want) {
			t.Errorf("%s: got %v, want %v", check.name, check.got, check.want)
		}
	}
}

// A default fills what is absent, not what is wrong: a value written out of
// bounds is still refused, with its name.
func TestAValueOutOfBoundsIsStillRefused(t *testing.T) {
	atRepoRoot(t)
	for _, bad := range []struct{ section, line, want string }{
		{"flight_phases", "enabled = true\ncruise_altitude_ft = 0", "cruise_altitude_ft must be positive"},
		{"adsb", "fetch_interval_seconds = 0", "invalid fetch interval"},
		{"wx", "refresh_interval_minutes = -1", "refresh_interval_minutes must be greater than 0"},
		{"logging", `level = "loud"`, "invalid log level"},
	} {
		cfg, err := Load(writeConfig(t, withLines(minimalConfig, bad.section, bad.line)))
		if err != nil {
			t.Fatal(err)
		}
		err = cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("[%s] %s: got %v, want %q", bad.section, bad.line, err, bad.want)
		}
	}
}

// withLines adds lines to the section of a TOML text, after the header when
// the section exists and as a new table otherwise (TOML refuses a table
// written twice).
func withLines(text, section, lines string) string {
	header := "[" + section + "]\n"
	if strings.Contains(text, header) {
		return strings.Replace(text, header, header+lines+"\n", 1)
	}
	return text + "\n" + header + lines + "\n"
}
