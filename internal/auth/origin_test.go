package auth

import (
	"net/http/httptest"
	"testing"
)

// Which pages may call the server, as a browser asks: the request's Host and
// the page's Origin. The server listens on 8000 and 8001.
func TestWhichPagesMayCallTheServer(t *testing.T) {
	o, err := NewOrigins("0.0.0.0", []int{8000, 8001}, []string{"https://Station.Example.ts.net/", "http://100.64.0.7:9000"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		host, origin string
		want         bool
		why          string
	}{
		{"station.lan:8000", "http://station.lan:8000", true, "the page itself"},
		{"station.lan:8001", "http://station.lan:8000", true, "the page reaching the audio on another port"},
		{"station.lan:8001", "https://station.lan:8000", true, "the same over https"},
		{"STATION.lan:8000", "http://station.LAN:8000", true, "names compare without case"},
		{"127.0.0.1:8000", "http://localhost:8001", true, "this machine's own names on a server port"},
		{"[::1]:8000", "http://[::1]:8001", true, "IPv6 loopback"},
		{"station.lan", "https://station.lan", true, "behind a proxy that keeps the Host header"},
		{"station.lan:8000", "https://station.example.ts.net", true, "listed, written with case and a slash"},
		{"station.lan:8000", "http://100.64.0.7:9000", true, "listed"},

		{"station.lan:8000", "http://evil.test", false, "a foreign page"},
		{"station.lan:8000", "http://evil.test:8000", false, "a foreign page on the server's port"},
		{"station.lan:8000", "http://station.lan:9999", false, "the server's name on a port it does not listen on"},
		{"station.lan:8000", "http://100.64.0.7:9001", false, "a listed host on another port"},
		{"station.lan:8000", "http://localhost:3000", false, "this machine's name on a port that is not the server's"},
		{"station.lan:8000", "null", false, "a sandboxed or file page"},
		{"station.lan:8000", "file://", false, "a file page"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "http://"+c.host+"/api/v1/aircraft", nil)
		r.Host = c.host
		r.Header.Set("Origin", c.origin)
		if got := o.Allowed(r); got != c.want {
			t.Errorf("%s: Host %s, Origin %s: allowed = %v, want %v", c.why, c.host, c.origin, got, c.want)
		}
		if got := o.AllowedOrAbsent(r); got != c.want {
			t.Errorf("%s: AllowedOrAbsent = %v, want %v", c.why, got, c.want)
		}
	}

	// No Origin: nothing to answer for CORS, and a client that is not a
	// browser for the WebSocket.
	r := httptest.NewRequest("GET", "http://station.lan:8000/api/v1/ws", nil)
	if o.Allowed(r) {
		t.Error("a request without Origin needs no CORS header")
	}
	if !o.AllowedOrAbsent(r) {
		t.Error("a client without Origin is not a browser and may open the WebSocket")
	}
}

// A specific bind address is one of the server's names.
func TestTheBindAddressIsOneOfTheServersNames(t *testing.T) {
	o, err := NewOrigins("192.0.2.10", []int{8000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://proxy.internal/api/v1/aircraft", nil)
	r.Header.Set("Origin", "http://192.0.2.10:8000")
	if !o.Allowed(r) {
		t.Error("the bind address on the server's port is the server")
	}
}

// What server.allowed_origins accepts, and what it refuses by name.
func TestAnAllowedOriginIsAnOriginAndNeverAStar(t *testing.T) {
	good := map[string]string{
		"http://100.64.0.7:8000":         "http://100.64.0.7:8000",
		"HTTPS://Mac.Tailnet.ts.net/":    "https://mac.tailnet.ts.net",
		"https://mac.tailnet.ts.net:443": "https://mac.tailnet.ts.net",
		"http://[FD7A::1]:8000":          "http://[fd7a::1]:8000",
	}
	for in, want := range good {
		got, err := NormalizeOrigin(in)
		if err != nil || got != want {
			t.Errorf("NormalizeOrigin(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"*", "", "mac.local:8000", "ftp://mac.local", "http://mac.local/page", "http://mac.local?x=1", "http://user@mac.local"} {
		if _, err := NormalizeOrigin(bad); err == nil {
			t.Errorf("NormalizeOrigin(%q) accepted", bad)
		}
		if _, err := NewOrigins("", []int{8000}, []string{bad}); err == nil {
			t.Errorf("NewOrigins accepted %q", bad)
		}
	}
}
