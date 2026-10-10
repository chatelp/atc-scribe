package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yegors/co-atc/internal/config"
)

// healthSaying is a sidecar that answers /health with the given JSON.
func healthSaying(t *testing.T, body string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

const degradedHealth = `{"status":"degraded","degraded":["fr"],"second_opinion":true,"second_opinion_ok":41,` +
	`"second_opinion_failed":2,"pid":4242,"models":{"en":{"id":"whisper-en","available":true},` +
	`"fr":{"id":"whisper-fr","available":false,"detail":"model directory not found"}}}`

// The settings panel's view of the running server: where the data goes and
// how much there is, what bounds the log, the live settings, who may sign in,
// and whether the sidecar is whole.
func TestTheServerStateSaysWhatIsRunning(t *testing.T) {
	s := newStack(t, stackOptions{sidecar: healthSaying(t, degradedHealth)})
	s.signIn()
	// Yesterday's file beside today's: it counts, and today's is listed first.
	yesterday := filepath.Join(s.cfg.Storage.SQLiteBasePath, "co-atc-2026-10-09.db")
	if err := os.WriteFile(yesterday, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.cfg.Storage.SQLiteBasePath, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var st ServerState
	s.get("/api/v1/server", &st)
	if st.Version != "0.1.0" || st.UptimeSeconds < 0 || st.Writable {
		t.Errorf("version %q uptime %d writable %v", st.Version, st.UptimeSeconds, st.Writable)
	}

	sto := st.Storage
	if sto.Dir != s.cfg.Storage.SQLiteBasePath || sto.ActiveDB != filepath.Join(sto.Dir, filepath.Base(sto.ActiveDB)) ||
		!strings.HasPrefix(filepath.Base(sto.ActiveDB), "co-atc-") {
		t.Errorf("storage dir %q active %q", sto.Dir, sto.ActiveDB)
	}
	if len(sto.DailyFiles) != 2 || !sto.DailyFiles[0].Active || sto.DailyFiles[0].Name != filepath.Base(sto.ActiveDB) ||
		sto.DailyFiles[1].Name != "co-atc-2026-10-09.db" || sto.DailyFiles[1].Bytes != 1000 || sto.DailyFiles[1].Active {
		t.Errorf("daily files = %+v, want today's (active) then yesterday's 1000 bytes, and not notes.txt", sto.DailyFiles)
	}
	if sto.ActiveDBBytes <= 0 || sto.TotalBytes != sto.ActiveDBBytes+1000 {
		t.Errorf("active %d bytes, total %d, want total = active + 1000", sto.ActiveDBBytes, sto.TotalBytes)
	}
	if !sto.Rotates || sto.RetentionGB != 20 || sto.GrowthBytesPerHour != -1 || sto.DaysUntilFull != -1 {
		t.Errorf("rotates %v retention %g growth %d days %g: want rotating, 20 GB, and 'still measuring'",
			sto.Rotates, sto.RetentionGB, sto.GrowthBytesPerHour, sto.DaysUntilFull)
	}
	if sto.DiskFreeBytes <= 0 || sto.DiskTotalBytes < sto.DiskFreeBytes {
		t.Errorf("disk free %d of %d", sto.DiskFreeBytes, sto.DiskTotalBytes)
	}

	if st.Logging.Level != "error" || st.Logging.Bounded || st.Logging.File != "" {
		t.Errorf("logging = %+v, want level error, stdout, unbounded", st.Logging)
	}
	if st.Settings.DBRetentionGB != 20 || st.Settings.LogLevel != "error" || st.Settings.ReferenceAirport != "LFPG" ||
		st.Settings.Matching == nil || !st.Settings.Matching.Letters || st.Settings.Matching.MinDigits != 3 {
		t.Errorf("settings = %+v", st.Settings)
	}
	if st.Access.Mode != "account" || !st.Access.LocalAllowed || len(st.Access.Accounts) != 1 || st.Access.Accounts[0] != testUser {
		t.Errorf("access = %+v", st.Access)
	}
	if st.Reference != nil {
		t.Errorf("reference = %+v, want none without reference data", st.Reference)
	}

	tr := st.Transcription
	if tr == nil {
		t.Fatal("no transcription state with a sidecar configured")
	}
	if !tr.Reachable || tr.Stale || tr.Status != "degraded" || len(tr.Degraded) != 1 || tr.Degraded[0] != "fr" ||
		tr.Detail != "whisper-fr: model directory not found" || !tr.SecondOpinion ||
		tr.SecondOpinionOK != 41 || tr.SecondOpinionFailed != 2 {
		t.Errorf("transcription = %+v", *tr)
	}
}

// A sidecar that does not answer: the last reading, marked stale, with the
// reason. (The state without any sidecar is checked with the settings below.)
func TestTheServerStateWithASidecarGone(t *testing.T) {
	gone := newStack(t, stackOptions{sidecar: "http://127.0.0.1:1"})
	gone.signIn()
	var st ServerState
	gone.get("/api/v1/server", &st)
	tr := st.Transcription
	if tr == nil || tr.Reachable || !tr.Stale || tr.Status != "unknown" || !strings.Contains(tr.Detail, "not reached just now") {
		t.Errorf("a sidecar that does not answer: %+v, want unreachable, stale, unknown, with the reason", tr)
	}
}

// What the panel can change while the server runs, what it refuses, and
// that a change outlives the request: it is written beside the configuration.
func TestServerSettingsAreAppliedPersistedAndRefused(t *testing.T) {
	s := newStack(t, stackOptions{})
	s.signIn()
	// The log level is a global: put it back for the tests after this one.
	t.Cleanup(func() { _ = s.runtime.Apply(s.runtime.Settings()) })

	put := func(body string) *httptest.ResponseRecorder { return s.do("PUT", "/api/v1/server/settings", body) }
	rec := put(`{"log_level":"debug"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("log_level debug: %d %s", rec.Code, rec.Body.String())
	}
	var got config.RuntimeSettings
	decode(t, rec, &got)
	if got.LogLevel != "debug" || got.DBRetentionGB != 20 || got.ReferenceAirport != "LFPG" || got.Matching == nil || got.Matching.MinDigits != 3 {
		t.Errorf("after log_level alone: %+v, want the other values kept", got)
	}
	var st ServerState
	s.get("/api/v1/server", &st)
	if st.Logging.Level != "debug" || st.Settings.LogLevel != "debug" {
		t.Errorf("the server state after the change: logging %+v settings %+v", st.Logging, st.Settings)
	}
	if st.Transcription != nil {
		t.Errorf("no sidecar: transcription = %+v, want absent", st.Transcription)
	}
	saved, err := os.ReadFile(filepath.Join(s.dir, config.RuntimeSettingsFile))
	if err != nil {
		t.Fatalf("the change was not written beside the configuration: %v", err)
	}
	var onDisk config.RuntimeSettings
	if err := json.Unmarshal(saved, &onDisk); err != nil || onDisk.LogLevel != "debug" || onDisk.DBRetentionGB != 20 {
		t.Errorf("on disk: %s (%v)", saved, err)
	}

	if rec := put(`{"db_retention_gb":5}`); rec.Code != http.StatusOK {
		t.Errorf("db_retention_gb 5: %d %s", rec.Code, rec.Body.String())
	}
	s.get("/api/v1/server", &st)
	if st.Storage.RetentionGB != 5 || st.Settings.DBRetentionGB != 5 || st.Settings.LogLevel != "debug" {
		t.Errorf("after the retention change: storage %g, settings %+v", st.Storage.RetentionGB, st.Settings)
	}

	rec = put(`{"matching":{"letters":false,"approx_operators":true,"min_digits":2,"context_digits":true,"sectors":true}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("matching: %d %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &got)
	m := got.Matching
	if m == nil || m.Letters || m.MinDigits != 2 || !m.ContextDigits || m.ContextLetters ||
		m.Sector["approach"].RadiusNM != 60 || m.Sector["ground"].MaxAltFt != 1500 {
		t.Errorf("matching = %+v, want the rules sent and the sector sizes filled in with the defaults", m)
	}

	for _, bad := range []struct{ name, body, says string }{
		{"a log level that does not exist", `{"log_level":"loud"}`, "log_level"},
		{"a retention under 1 GB", `{"db_retention_gb":0.5}`, "db_retention_gb"},
		{"a retention over 1000 GB", `{"db_retention_gb":5000}`, "db_retention_gb"},
		{"five-digit flight numbers", `{"matching":{"min_digits":7}}`, "min_digits"},
		{"a sector of 1 000 NM", `{"matching":{"min_digits":3,"sector":{"tower":{"radius_nm":1000,"max_alt_ft":6000}}}}`, "radius_nm"},
		{"a sector kind that does not exist", `{"matching":{"min_digits":3,"sector":{"centre":{"radius_nm":60,"max_alt_ft":6000}}}}`, "unknown kind"},
		{"another reference airport, with no reference data to check it", `{"reference_airport":"LFPO"}`, "reference_airport"},
		{"a further airport, likewise", `{"also_airports":["LFPO"]}`, "reference_airport"},
		{"not JSON", `{`, "invalid settings"},
	} {
		rec := put(bad.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), bad.says) {
			t.Errorf("%s: %d %q, want 400 mentioning %q", bad.name, rec.Code, strings.TrimSpace(rec.Body.String()), bad.says)
		}
	}
	// Nothing refused was applied.
	s.get("/api/v1/server", &st)
	if st.Settings.LogLevel != "debug" || st.Settings.DBRetentionGB != 5 || st.Settings.ReferenceAirport != "LFPG" ||
		st.Settings.Matching.MinDigits != 2 {
		t.Errorf("after the refusals: %+v", st.Settings)
	}
}

// The first-run question on an empty directory, from the page's side.
func TestTheFirstRunQuestionOnAnEmptyDirectory(t *testing.T) {
	s := newStack(t, stackOptions{noAccount: true})
	var st SetupState
	s.get("/api/v1/setup/status", &st)
	if !st.Needed || !st.LocalOnly || !st.LocalAllowed || st.Host != "127.0.0.1" ||
		st.File != filepath.Join(s.dir, "users.json") {
		t.Errorf("setup status = %+v", st)
	}
	if _, err := os.Stat(st.File); !os.IsNotExist(err) {
		t.Errorf("the accounts file exists before any answer: %v", err)
	}
	if rec := s.do("POST", "/api/v1/setup", `{"mode":"nowhere"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("an unknown mode: %d, want 400", rec.Code)
	}
	if rec := s.do("POST", "/api/v1/setup", `{`); rec.Code != http.StatusBadRequest {
		t.Errorf("a malformed answer: %d, want 400", rec.Code)
	}
	if rec := s.do("POST", "/api/v1/setup", `{"mode":"account","username":"pierre","password":"short"}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "10 characters") {
		t.Errorf("a short password: %d %s, want 400 saying how long", rec.Code, rec.Body.String())
	}
	s.get("/api/v1/setup/status", &st)
	if !st.Needed {
		t.Fatal("a refused answer counted as an answer")
	}

	rec := s.do("POST", "/api/v1/setup", `{"mode":"local"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"mode":"local"`) {
		t.Fatalf("choosing local: %d %s", rec.Code, rec.Body.String())
	}
	s.get("/api/v1/setup/status", &st)
	if st.Needed {
		t.Error("after choosing local the question is still asked")
	}
	if b, err := os.ReadFile(st.File); err != nil || !strings.Contains(string(b), `"local"`) {
		t.Errorf("the choice is not recorded in %s: %s (%v)", st.File, b, err)
	}
	if rec := s.do("POST", "/api/v1/setup", `{"mode":"local"}`); rec.Code != http.StatusConflict {
		t.Errorf("answering twice: %d, want 409", rec.Code)
	}
	// With an account there is nothing to set up, and the page cannot add one.
	withAccount := newStack(t, stackOptions{})
	withAccount.get("/api/v1/setup/status", &st)
	if st.Needed {
		t.Error("an account exists and the question is asked")
	}
	if rec := withAccount.do("POST", "/api/v1/setup", `{"mode":"local"}`); rec.Code != http.StatusConflict {
		t.Errorf("setup with an account: %d, want 409", rec.Code)
	}
	// Bound to every interface, local-only is not an answer.
	open := newStack(t, stackOptions{noAccount: true, host: "0.0.0.0"})
	open.get("/api/v1/setup/status", &st)
	if !st.Needed || st.LocalOnly || st.LocalAllowed || st.Host != "0.0.0.0" {
		t.Errorf("on 0.0.0.0: %+v", st)
	}
	if rec := open.do("POST", "/api/v1/setup", `{"mode":"local"}`); rec.Code != http.StatusConflict {
		t.Errorf("local on 0.0.0.0: %d, want 409", rec.Code)
	}
}
