package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"github.com/yegors/co-atc/internal/auth"
	"github.com/yegors/co-atc/internal/websocket"
)

// dialWS opens /ws on the real port with the session held, as the page does.
// It returns the handshake's status code with the error, so a refusal can be
// told from a failure.
func (s *stack) dialWS() (*gorilla.Conn, int, error) {
	s.t.Helper()
	h := http.Header{}
	if s.cookie != nil {
		h.Set("Cookie", auth.CookieName+"="+s.cookie.Value)
	}
	conn, resp, err := gorilla.DefaultDialer.Dial(s.wsURL(), h)
	code := 0
	if resp != nil {
		code = resp.StatusCode
	}
	if err != nil {
		return nil, code, err
	}
	s.t.Cleanup(func() { conn.Close() })
	return conn, code, nil
}

// connectWS is dialWS when the connection is expected to open.
func (s *stack) connectWS() *gorilla.Conn {
	s.t.Helper()
	conn, code, err := s.dialWS()
	if err != nil {
		s.t.Fatalf("dialing /ws: %v (status %d)", err, code)
	}
	return conn
}

// waitForMessage reads until a message of the given type arrives, skipping
// the broadcasts the services send meanwhile (status updates, frequency
// status), and fails after the timeout.
func waitForMessage(t *testing.T, conn *gorilla.Conn, typ string, timeout time.Duration) websocket.Message {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for a %q message: %v", typ, err)
		}
		var m websocket.Message
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("a message that is not JSON: %q", data)
		}
		if m.Type == typ {
			return m
		}
	}
}

func sendMessage(t *testing.T, conn *gorilla.Conn, typ string, data map[string]any) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"type": typ, "data": data})
	if err := conn.WriteMessage(gorilla.TextMessage, b); err != nil {
		t.Fatal(err)
	}
}
