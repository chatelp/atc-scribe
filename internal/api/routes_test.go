package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/pkg/logger"
)

// The routes the server mounts, frozen in testdata/routes.txt: method, path, the
// function behind it, and whether it sits behind the sign-in lock. Moving
// handlers between files must not add, drop or rename one, nor wire one to
// another function: the page, radio-ctl-sync, the sidecar and the tools call
// them by these exact paths. A deliberate change to the routes rewrites the file
// with ATC_WRITE_ROUTES=1 and shows in the diff.
func TestTheMountedRoutesAreTheFrozenList(t *testing.T) {
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}
	cfg.Server.Host = "127.0.0.1"

	// Through the real constructor, with no services behind it: mounting the
	// routes reads none of them.
	router := NewRouter(nil, nil, nil, nil, cfg, log, nil, sqlite.NewTranscriptionStorage(nil, log), nil)

	// A route with more middleware than the root's sits in the group behind the
	// sign-in lock.
	routes := router.Routes().(chi.Routes)
	root := len(routes.Middlewares())
	var lines []string
	walk := func(method, route string, h http.Handler, mws ...func(http.Handler) http.Handler) error {
		access := "open"
		if len(mws) > root {
			access = "locked"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s %s", method, route, handlerName(h), access))
		return nil
	}
	if err := chi.Walk(routes, walk); err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"

	const frozen = "testdata/routes.txt"
	if os.Getenv("ATC_WRITE_ROUTES") == "1" {
		if err := os.WriteFile(frozen, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(frozen)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("the mounted routes differ from %s\n--- mounted\n%s--- frozen\n%s", frozen, got, want)
	}
}

// handlerName names the function a route ends in, past the middleware a group
// wraps it in: "(*Handler).GetAllAircraft", or the type of a handler that is
// not a function.
func handlerName(h http.Handler) string {
	if chain, ok := h.(*chi.ChainHandler); ok {
		h = chain.Endpoint
	}
	if f, ok := h.(http.HandlerFunc); ok {
		name := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
		name = name[strings.LastIndex(name, "/")+1:]
		name = strings.TrimPrefix(name, "api.")
		return strings.TrimSuffix(name, "-fm")
	}
	return reflect.TypeOf(h).String()
}
