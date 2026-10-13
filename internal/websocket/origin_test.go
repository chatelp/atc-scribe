package websocket

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func dialFrom(t *testing.T, url, origin string) (int, error) {
	t.Helper()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(url, h)
	if conn != nil {
		conn.Close()
	}
	code := 0
	if resp != nil {
		code = resp.StatusCode
	}
	return code, err
}

// A browser sends the cookie with a WebSocket handshake whatever page asks:
// the hub accepts no Origin (not a browser) and its own origin, and refuses
// the others, without being told anything.
func TestTheHubRefusesAForeignPageByDefault(t *testing.T) {
	_, url := newTestServer(t)
	own := "http" + strings.TrimPrefix(url, "ws") // the test server's own origin

	if _, err := dialFrom(t, url, ""); err != nil {
		t.Errorf("no Origin (a client that is not a browser): %v", err)
	}
	if _, err := dialFrom(t, url, own); err != nil {
		t.Errorf("its own origin %s: %v", own, err)
	}
	if code, err := dialFrom(t, url, "http://evil.test"); err == nil || code != http.StatusForbidden {
		t.Errorf("a foreign page: err %v, status %d, want a refused handshake with 403", err, code)
	}
}

// With a check set, the check decides.
func TestTheHubAsksTheCheckItIsGiven(t *testing.T) {
	s, url := newTestServer(t)
	s.SetOriginCheck(func(r *http.Request) bool {
		o := r.Header.Get("Origin")
		return o == "" || o == "https://page.example"
	})
	if _, err := dialFrom(t, url, "https://page.example"); err != nil {
		t.Errorf("an allowed page: %v", err)
	}
	if _, err := dialFrom(t, url, ""); err != nil {
		t.Errorf("no Origin: %v", err)
	}
	if code, err := dialFrom(t, url, "http://evil.test"); err == nil || code != http.StatusForbidden {
		t.Errorf("a page the check refuses: err %v, status %d, want 403", err, code)
	}
}
