package api

import (
	"net/http"
	"strings"
	"testing"

	gorilla "github.com/gorilla/websocket"

	"github.com/yegors/co-atc/internal/auth"
)

// The WebSocket takes the same pages as CORS, and the clients that send no
// Origin: a foreign page holding the user's cookie is refused at the
// handshake.
func TestTheWebSocketTakesTheSamePagesAsCORS(t *testing.T) {
	s := newStack(t, stackOptions{})
	s.signIn()
	port := s.srv.URL[strings.LastIndex(s.srv.URL, ":")+1:]

	dial := func(origin string) (int, error) {
		h := http.Header{}
		h.Set("Cookie", auth.CookieName+"="+s.cookie.Value)
		if origin != "" {
			h.Set("Origin", origin)
		}
		conn, resp, err := gorilla.DefaultDialer.Dial(s.wsURL(), h)
		if conn != nil {
			conn.Close()
		}
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		return code, err
	}

	for _, origin := range []string{"", "http://127.0.0.1:" + port, "http://localhost:8000", "http://127.0.0.1:8001"} {
		if _, err := dial(origin); err != nil {
			t.Errorf("Origin %q: %v, want the connection open", origin, err)
		}
	}
	for _, origin := range []string{"http://evil.test", "http://evil.test:" + port, "http://localhost:3000", "null"} {
		if code, err := dial(origin); err == nil || code != http.StatusForbidden {
			t.Errorf("Origin %q: err %v, status %d, want a refused handshake with 403", origin, err, code)
		}
	}
}
