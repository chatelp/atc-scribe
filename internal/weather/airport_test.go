package weather

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// A weather API that answers NOTAMs naming the airport asked for. The previous
// airport's answer can be held back to arrive after a change of airport -- the
// race this test is about.
type fakeAPI struct {
	mu      sync.Mutex
	holdFor string
	release chan struct{}
}

func (f *fakeAPI) handler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	kind, code := parts[0], parts[len(parts)-1]
	f.mu.Lock()
	hold := f.holdFor == code
	f.mu.Unlock()
	if hold {
		<-f.release
	}
	if kind != "notams" {
		w.WriteHeader(http.StatusNoContent) // what LFPZ answers for METAR and TAF
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `[{"id":"%s-1","text":"NOTAM for %s"}]`, code, code)
}

func newTestService(t *testing.T, api *fakeAPI, code string) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(api.handler))
	t.Cleanup(srv.Close)
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	return NewService(ConfigWeatherConfig{
		RefreshIntervalMinutes: 60, APIBaseURL: srv.URL, RequestTimeoutSeconds: 5,
		MaxRetries: 0, FetchMETAR: true, FetchTAF: true, FetchNOTAMs: true, CacheExpiryMinutes: 60,
	}, code, log)
}

// cached reads the cache itself. GetWeatherData would wait up to 30 s for a
// start-up signal these tests have no reason to send, and then return an error
// value in which "no LFPZ" is true for the wrong reason.
func cached(s *Service) *WeatherData { return s.cache.Get() }

func notamText(d *WeatherData) string {
	if d == nil || d.NOTAMs == nil {
		return ""
	}
	return fmt.Sprint(d.NOTAMs)
}

func TestSetAirportEmptiesTheCache(t *testing.T) {
	api := &fakeAPI{}
	s := newTestService(t, api, "LFPZ")
	s.fetchAndUpdateCache()
	if !strings.Contains(notamText(cached(s)), "LFPZ") {
		t.Fatalf("precondition: LFPZ's NOTAMs should be cached, got %q", notamText(cached(s)))
	}

	s.SetAirport("LFPO") // not started: no refetch, so only the emptying is observed
	if d := cached(s); d != nil {
		t.Errorf("after switching to LFPO the cache should be empty, got NOTAMs %q", notamText(d))
	}
	if s.AirportCode() != "LFPO" {
		t.Errorf("airport should be LFPO, got %s", s.AirportCode())
	}
}

func TestAFetchForThePreviousAirportIsDiscarded(t *testing.T) {
	api := &fakeAPI{holdFor: "LFPZ", release: make(chan struct{})}
	s := newTestService(t, api, "LFPZ")

	done := make(chan struct{})
	go func() { s.fetchAndUpdateCache(); close(done) }() // stuck in LFPZ's request

	time.Sleep(50 * time.Millisecond)
	s.SetAirport("LFPO")
	close(api.release) // LFPZ's answer arrives after the change
	<-done

	if d := cached(s); d != nil {
		t.Fatalf("a result fetched for LFPZ was filed under LFPO: %q", notamText(d))
	}
	s.fetchAndUpdateCache()
	if got := notamText(cached(s)); !strings.Contains(got, "LFPO") {
		t.Errorf("LFPO's own NOTAMs should now be cached, got %q", got)
	}
}
