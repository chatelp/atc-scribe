package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"github.com/yegors/co-atc/internal/websocket"
)

// bulkRequest asks for the aircraft the way the page does on load, with the
// filters given, and returns what came back.
func bulkRequest(t *testing.T, conn *gorilla.Conn, filters map[string]any) (hexes []string, count int, counts map[string]any) {
	t.Helper()
	sendMessage(t, conn, websocket.MessageTypeAircraftBulkRequest, map[string]any{"filters": filters})
	m := waitForMessage(t, conn, websocket.MessageTypeAircraftBulkResponse, 5*time.Second)
	list, _ := m.Data["aircraft"].([]any)
	for _, a := range list {
		if obj, ok := a.(map[string]any); ok {
			hexes = append(hexes, obj["hex"].(string))
		}
	}
	n, _ := m.Data["count"].(float64)
	counts, _ = m.Data["counts"].(map[string]any)
	return hexes, int(n), counts
}

func has(hexes []string, hex string) bool {
	for _, h := range hexes {
		if h == hex {
			return true
		}
	}
	return false
}

// The hub itself: who may connect, what it does with what it does not
// understand, and that a page gone does not hold the others. One stack.
func TestTheWebSocketHub(t *testing.T) {
	shared := newStack(t, stackOptions{})

	// /ws is behind the lock like the data it carries: the handshake itself
	// is refused without a session.
	t.Run("takes a session", func(t *testing.T) {
		s := shared.with(t)
		if _, code, err := s.dialWS(); err == nil || code != http.StatusUnauthorized {
			t.Errorf("dialing /ws without a session: err %v, status %d, want a refused handshake with 401", err, code)
		}
		shared.signIn()
		s = shared.with(t) // the copy was taken before the session existed
		conn := s.connectWS()
		if err := conn.WriteControl(gorilla.PingMessage, nil, time.Now().Add(time.Second)); err != nil {
			t.Errorf("the connection is not open: %v", err)
		}
	})

	// What the hub does with what it does not understand: nothing, and the
	// connection stays open for what comes next.
	t.Run("odd messages do not close the connection", func(t *testing.T) {
		s := shared.with(t)
		conn := s.connectWS()
		if err := conn.WriteMessage(gorilla.TextMessage, []byte("not json")); err != nil {
			t.Fatal(err)
		}
		sendMessage(t, conn, "filter_update", map[string]any{"anything": true})
		sendMessage(t, conn, "simulation_control_update", map[string]any{})
		sendMessage(t, conn, websocket.MessageTypeAircraftBulkRequest, nil) // no filters at all
		m := waitForMessage(t, conn, websocket.MessageTypeAircraftBulkResponse, 5*time.Second)
		if n, _ := m.Data["count"].(float64); int(n) != aircraftSeen {
			t.Errorf("after the odd messages, a bulk request answered %v", m.Data["count"])
		}
	})

	// A page gone away is forgotten: what the server broadcasts afterwards
	// is not held for it, and the other pages keep receiving.
	t.Run("a closed page does not stop the broadcasts", func(t *testing.T) {
		s := shared.with(t)
		gone := s.connectWS()
		stays := s.connectWS()
		gone.Close()
		time.Sleep(50 * time.Millisecond)
		s.ws.Broadcast(&websocket.Message{Type: "frequency_status", Data: map[string]any{"id": "mix", "status": "ok"}})
		m := waitForMessage(t, stays, "frequency_status", 5*time.Second)
		if m.Data["id"] != "mix" {
			t.Errorf("the broadcast reached the open page as %v", m.Data)
		}
	})
}

// The page's first message, and the answer it builds the map from, on the
// aircraft stack (the cruise aircraft is in CRZ).
func TestBulkRequests(t *testing.T) {
	shared := aircraftStack(t)

	t.Run("answered to the client that asked", func(t *testing.T) {
		s := shared.with(t)
		conn := s.connectWS()
		hexes, count, counts := bulkRequest(t, conn, map[string]any{"last_seen_minutes": 10})
		if count != aircraftSeen || len(hexes) != aircraftSeen {
			t.Fatalf("count %d with %v, want %d aircraft", count, hexes, aircraftSeen)
		}
		if counts["ground_total"] != 1.0 || counts["air_total"] != 3.0 || counts["air_active"] != 3.0 {
			t.Errorf("counts = %v", counts)
		}
		// Only the asking client gets it: a second page sees nothing of it.
		other := s.connectWS()
		hexes, _, _ = bulkRequest(t, conn, map[string]any{})
		if len(hexes) != aircraftSeen {
			t.Errorf("no filters: %v", hexes)
		}
		_ = other.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		for {
			_, data, err := other.ReadMessage()
			if err != nil {
				break // nothing more, which is the point
			}
			if strings.Contains(string(data), `"type":"`+websocket.MessageTypeAircraftBulkResponse+`"`) {
				t.Fatal("a bulk response reached a client that did not ask for it")
			}
		}
	})

	// The filters the bulk request understands, each on its own.
	t.Run("the filters", func(t *testing.T) {
		s := shared.with(t)
		conn := s.connectWS()
		hexes, _, _ := bulkRequest(t, conn, map[string]any{"phases": []string{"CRZ"}})
		if len(hexes) != 1 || hexes[0] != hexCruise {
			t.Errorf("phases=[CRZ]: %v, want the cruise aircraft alone", hexes)
		}
		hexes, _, _ = bulkRequest(t, conn, map[string]any{"phases": []string{"APP", "T/D"}})
		if len(hexes) != 0 {
			t.Errorf("phases=[APP T/D]: %v, want none (an aircraft without a phase does not match)", hexes)
		}
		hexes, _, _ = bulkRequest(t, conn, map[string]any{"show_ground": false})
		if len(hexes) != aircraftSeen-1 || has(hexes, hexGround) {
			t.Errorf("show_ground=false: %v, want the ground aircraft gone", hexes)
		}
		hexes, _, _ = bulkRequest(t, conn, map[string]any{"show_air": false})
		if len(hexes) != 1 || hexes[0] != hexGround {
			t.Errorf("show_air=false: %v, want the ground aircraft alone", hexes)
		}
		hexes, count, counts := bulkRequest(t, conn, map[string]any{"show_air": false, "show_ground": false})
		if len(hexes) != 0 || count != 0 || counts["air_total"] != 0.0 {
			t.Errorf("neither air nor ground: %v, count %d, counts %v", hexes, count, counts)
		}
		hexes, _, _ = bulkRequest(t, conn, map[string]any{"status": []string{"signal_lost"}})
		if len(hexes) != 0 {
			t.Errorf("status=[signal_lost]: %v, want none", hexes)
		}
		hexes, _, _ = bulkRequest(t, conn, map[string]any{"exclude_other_airports_grounded": true})
		if len(hexes) != aircraftSeen-1 || has(hexes, hexGround) {
			t.Errorf("exclude_other_airports_grounded: %v, want the ground aircraft (13 NM out) gone", hexes)
		}
	})
}
