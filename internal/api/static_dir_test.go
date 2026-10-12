package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/pkg/logger"
)

// The page is served from server.static_dir when it is set. Production points
// it at a snapshot of www/ taken at start (12/10): until then, production and
// the working tree were the same directory, and an edit was live on reload.
func TestThePageIsServedFromTheConfiguredDirectory(t *testing.T) {
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	pages := t.TempDir()
	if err := os.WriteFile(filepath.Join(pages, "probe.txt"), []byte("<title>snapshot</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.StaticDir = pages
	router := NewRouter(nil, nil, nil, nil, cfg, log, nil, sqlite.NewTranscriptionStorage(nil, log), nil)

	rec := httptest.NewRecorder()
	router.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe.txt", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "<title>snapshot</title>" {
		t.Fatalf("GET /probe.txt from static_dir: %d %q, want 200 and the snapshot's page", rec.Code, rec.Body.String())
	}

	// Unset, the directory is www next to the working directory, as it always was.
	cfg.Server.StaticDir = ""
	router = NewRouter(nil, nil, nil, nil, cfg, log, nil, sqlite.NewTranscriptionStorage(nil, log), nil)
	rec = httptest.NewRecorder()
	router.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe.txt", nil))
	if rec.Code == http.StatusOK && rec.Body.String() == "<title>snapshot</title>" {
		t.Fatal("static_dir unset still served the snapshot: the default is not www")
	}
}
