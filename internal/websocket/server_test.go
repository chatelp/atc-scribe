package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(log)
	go s.Run()
	ts := httptest.NewServer(http.HandlerFunc(s.HandleConnection))
	t.Cleanup(ts.Close)
	return s, "ws" + strings.TrimPrefix(ts.URL, "http")
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func waitForClients(t *testing.T, s *Server, n int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		s.mu.RLock()
		got := len(s.clients)
		s.mu.RUnlock()
		if got == n {
			return
		}
	}
	t.Fatalf("expected %d registered clients", n)
}

// A client that stops reading -- a phone asleep, a VPN switching -- must not
// stop the others, nor the caller of Broadcast, which for aircraft updates is
// the ADS-B loop itself. Before the fix, the writer held the client's lock
// while blocked on the network, and Broadcast froze after about 0.5 MB.
func TestAClientThatStopsReadingDoesNotStopTheOthers(t *testing.T) {
	s, url := newTestServer(t)
	_ = dial(t, url) // never reads
	healthy := dial(t, url)
	waitForClients(t, s, 2)

	const n = 400
	last := make(chan struct{})
	go func() {
		for {
			_, data, err := healthy.ReadMessage()
			if err != nil {
				return
			}
			var m Message
			if json.Unmarshal(data, &m) == nil && m.Data["seq"] == float64(n-1) {
				close(last)
				return
			}
		}
	}()

	pad := strings.Repeat("x", 8<<10) // 3.2 MB in all, well past the 0.5 MB that froze
	sent := make(chan struct{})
	go func() {
		for i := 0; i < n; i++ {
			s.Broadcast(&Message{Type: MessageTypeAircraftUpdate, Data: map[string]interface{}{"seq": i, "pad": pad}})
			time.Sleep(time.Millisecond)
		}
		close(sent)
	}()

	select {
	case <-sent:
	case <-time.After(10 * time.Second):
		t.Fatal("Broadcast is blocked behind a client that stopped reading")
	}
	select {
	case <-last:
	case <-time.After(5 * time.Second):
		t.Fatal("the client that reads stopped receiving")
	}
}

// Queued messages (everything but aircraft movements) go through Run, which
// must not send on a channel closed in between: it would panic.
func TestQueuedMessagesReachAClient(t *testing.T) {
	s, url := newTestServer(t)
	c := dial(t, url)
	waitForClients(t, s, 1)
	s.Broadcast(&Message{Type: MessageTypeFrequenciesChanged, Data: map[string]interface{}{"id": "124350"}})
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var m Message
	if err := json.Unmarshal(data, &m); err != nil || m.Type != MessageTypeFrequenciesChanged {
		t.Fatalf("got %s (%v)", data, err)
	}
}
