package api

import (
	"net/http"
	"testing"
)

// A page from another origin is given no CORS header at all -- not a 403:
// the answer is still sent, and the browser keeps it from the page. The
// page's own origins keep theirs (stream_test.go).
func TestAForeignPageGetsNoCORSHeader(t *testing.T) {
	s := newStack(t, stackOptions{})
	s.signIn()
	const foreign = "http://evil.test"

	noCORS := func(h http.Header, what string) {
		t.Helper()
		for _, k := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials",
			"Access-Control-Allow-Methods", "Access-Control-Allow-Headers"} {
			if v := h.Get(k); v != "" {
				t.Errorf("%s: %s = %q, want none for %s", what, k, v, foreign)
			}
		}
	}

	rec := s.do("GET", "/api/v1/aircraft", "", func(r *http.Request) { r.Header.Set("Origin", foreign) })
	if rec.Code != http.StatusOK {
		t.Errorf("GET /aircraft from a foreign page: %d, want 200 (no header, not a refusal)", rec.Code)
	}
	noCORS(rec.Header(), "GET /aircraft")

	rec = s.do("OPTIONS", "/api/v1/aircraft", "", func(r *http.Request) {
		r.Header.Set("Origin", foreign)
		r.Header.Set("Access-Control-Request-Method", "PUT")
	})
	noCORS(rec.Header(), "preflight")

	rec = s.do("POST", "/api/v1/auth/login", `{"name":"x","password":"y"}`, func(r *http.Request) { r.Header.Set("Origin", foreign) })
	noCORS(rec.Header(), "POST /auth/login")

	// On the real port too, where the page's audio is fetched from.
	req, _ := http.NewRequest("GET", s.srv.URL+"/api/v1/health", nil)
	req.Header.Set("Origin", foreign)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	noCORS(resp.Header, "GET /health on a real port")

	// The same request from the server's own page is answered for it.
	rec = s.do("GET", "/api/v1/aircraft", "", func(r *http.Request) { r.Header.Set("Origin", "http://localhost:8001") })
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8001" {
		t.Errorf("the server's own page on its other port: Access-Control-Allow-Origin = %q", got)
	}
	if v := rec.Header().Get("Vary"); v == "" {
		t.Error("an answer that depends on Origin says Vary: Origin")
	}
}

// server.allowed_origins adds pages the server cannot know about: the address
// a VPN gives it, a proxy that rewrites the Host header.
func TestAnOriginListedInTheConfigurationIsAnswered(t *testing.T) {
	s := newStack(t, stackOptions{})
	s.cfg.Server.AllowedOrigins = []string{"https://mac.tailnet.ts.net"}
	r := &Router{handler: s.handler, middleware: NewMiddleware(s.log), config: s.cfg, logger: s.log}
	s.routes = r.Routes()
	s.signIn()

	rec := s.do("GET", "/api/v1/aircraft", "", func(r *http.Request) { r.Header.Set("Origin", "https://mac.tailnet.ts.net") })
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://mac.tailnet.ts.net" {
		t.Errorf("listed origin: Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("listed origin: Access-Control-Allow-Credentials = %q", got)
	}
}
