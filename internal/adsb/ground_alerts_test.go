package adsb

import (
	"reflect"
	"testing"
	"time"

	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// Only the two methods the alerts read; the rest of the Storage is nil and
// would panic if touched, which is the point.
type alertStorage struct {
	Storage
	aircraft map[string]*Aircraft
	history  map[string][]PhaseChange
}

func (s alertStorage) GetByHex(hex string) (*Aircraft, bool) {
	a, ok := s.aircraft[hex]
	return a, ok
}

func (s alertStorage) GetPhaseHistory(hex string) ([]PhaseChange, error) {
	return s.history[hex], nil
}

type captureWS struct{ msgs []*websocket.Message }

func (c *captureWS) Broadcast(m *websocket.Message) { c.msgs = append(c.msgs, m) }

// A takeoff and a landing each reach the page as one phase_change message
// with the same fields; other phases and unknown aircraft send nothing.
func TestGroundTransitionsReachThePage(t *testing.T) {
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	off := &Aircraft{Hex: "aaa111", Flight: "AFR123", ADSB: &ADSBTarget{AltBaro: 300}}
	down := &Aircraft{Hex: "bbb222", Flight: "EZY77", OnGround: true, ADSB: &ADSBTarget{AltBaro: 0}}
	st := alertStorage{
		aircraft: map[string]*Aircraft{"aaa111": off, "bbb222": down, "ccc333": {Hex: "ccc333", ADSB: &ADSBTarget{}}},
		history: map[string][]PhaseChange{
			"aaa111": {{Phase: "T/O"}, {Phase: "TAX"}},
			"bbb222": {{Phase: "T/D"}, {Phase: "ARR"}},
		},
	}
	ws := &captureWS{}
	s := &Service{storage: st, wsServer: ws, logger: log}

	s.sendImmediateGroundTransitionAlerts([]PhaseChangeInsert{
		{Hex: "aaa111", Phase: "T/O", Timestamp: at, Airport: "LFPG"},
		{Hex: "ccc333", Phase: "CRZ", Timestamp: at},
		{Hex: "nobody", Phase: "T/D", Timestamp: at},
		{Hex: "bbb222", Phase: "T/D", Timestamp: at.Add(time.Minute), Airport: "LFPO"},
	})

	want := []map[string]interface{}{
		{"hex": "aaa111", "flight": "AFR123", "phase": "T/O", "prev_phase": "TAX", "transition": "TAX → T/O",
			"airport": "LFPG", "altitude": 300.0, "on_ground": false, "timestamp": "2026-10-12T09:30:00Z"},
		{"hex": "bbb222", "flight": "EZY77", "phase": "T/D", "prev_phase": "ARR", "transition": "ARR → T/D",
			"airport": "LFPO", "altitude": 0.0, "on_ground": true, "timestamp": "2026-10-12T09:31:00Z"},
	}
	if len(ws.msgs) != len(want) {
		t.Fatalf("%d messages, want %d", len(ws.msgs), len(want))
	}
	for i, m := range ws.msgs {
		if m.Type != "phase_change" {
			t.Errorf("message %d: type %q, want phase_change", i, m.Type)
		}
		if !reflect.DeepEqual(m.Data, want[i]) {
			t.Errorf("message %d:\n got %#v\nwant %#v", i, m.Data, want[i])
		}
	}

	// Without a page to tell, nothing breaks.
	s.wsServer = nil
	s.sendImmediateGroundTransitionAlerts([]PhaseChangeInsert{{Hex: "aaa111", Phase: "T/O", Timestamp: at}})
}
