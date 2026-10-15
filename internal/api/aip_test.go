package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/aip"
)

// toServer sends every request to a test server, keeping its path: the SIA.
type toServer struct{ base string }

func (s toServer) RoundTrip(req *http.Request) (*http.Response, error) {
	target, _ := http.NewRequest(req.Method, s.base+req.URL.Path, nil)
	return http.DefaultTransport.RoundTrip(target)
}

const fakeENR44 = `<table><tr><td>ELCOB</td><td>49°10'00.0"N 001°50'00.0"E</td></tr></table>`

// The settings page's list of official names: absent, made on the button
// with a session, then up to date; the switch kept with the settings.
func TestTheOfficialNamesAreMadeFromThePanel(t *testing.T) {
	s := newStack(t, stackOptions{})
	sia := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "FR-ENR-4.4-fr-FR.html") {
			w.Write([]byte(fakeENR44))
			return
		}
		w.Write([]byte(`<p>DE GAULLE Approche. Departure OKIPA 1W.</p>`))
	}))
	t.Cleanup(sia.Close)
	s.cfg.Station.Latitude, s.cfg.Station.Longitude = 49.0, 2.5
	maker := &aip.Maker{OutPath: filepath.Join(s.dir, "aip-names.json"), CacheDir: filepath.Join(s.dir, "aip-cache"),
		Client: &http.Client{Transport: toServer{sia.URL}}}
	var made *aip.Names
	s.handler.AttachAIP(maker, func(n *aip.Names) { made = n })

	// Without a session, nothing reaches the SIA.
	if rec := s.do("POST", "/api/v1/aip-names/refresh", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh without a session: %d", rec.Code)
	}
	s.signIn()
	var st aipState
	s.get("/api/v1/aip-names", &st)
	if st.Present || st.InForce == "" || st.AutoUpdate || st.RadiusNM != AIPRadiusNM {
		t.Fatalf("before: %+v", st)
	}

	if rec := s.do("POST", "/api/v1/aip-names/refresh", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
	}
	for i := 0; i < 200 && maker.State().Refreshing; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	s.get("/api/v1/aip-names", &st)
	if !st.Present || st.Points != 1 || st.Procedures != 1 || st.AIRAC != st.InForce || st.LastError != "" ||
		!strings.Contains(st.Source, "SIA (DGAC)") {
		t.Fatalf("after: %+v", st)
	}
	if made == nil || made.Points[0].Name != "ELCOB" {
		t.Errorf("the matcher was not told of the list: %+v", made)
	}

	if rec := s.do("PUT", "/api/v1/server/settings", `{"aip_names":{"auto_update":true}}`); rec.Code != http.StatusOK {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body.String())
	}
	s.get("/api/v1/aip-names", &st)
	if !st.AutoUpdate {
		t.Error("the switch did not hold")
	}
}
