package adsb

// The live prediction: where each tracked aircraft should be now, between
// two polls of the receiver, published to the page once a second as an
// aircraft_predicted_state message. The page draws an aircraft from what the
// receiver said; this only says where the server expects it to be meanwhile,
// and a real update always wins over it.
//
// What it rests on:
//   - the trajectory forecast (trajectory_prediction.go), interpolated at the
//     time asked, from the position of the latest snapshot to the first
//     forecast point, then between forecast points;
//   - anchored on when the position was decoded (seen_pos), not on the poll
//     that repeated it: a receiver giving a position every ~1.7 s, polled
//     every second, repeats the fix at half the polls (D79, 13/10/2026);
//   - nothing for an aircraft whose position is older than
//     maxLivePredictionAge, nor below minPredictionConfidence.
//
// The message's form is pinned by TestMotionMessagesKeepTheirForm.

import (
	"context"
	"math"
	"time"

	"github.com/yegors/co-atc/internal/websocket"
)

const (
	// livePredictionInterval is how often the predicted states go out: once
	// a second, between two polls at the same rate.
	livePredictionInterval = 1 * time.Second
	// minPredictionConfidence is the floor below which a predicted state is
	// not sent; the forecast's confidence decays with time, turn and
	// acceleration (forecastPoints). The page applies the same floor.
	minPredictionConfidence = 0.65
	// maxLivePredictionAge: no prediction from a position decoded longer ago
	// than this; aircraft.json drops a position after a minute.
	maxLivePredictionAge = 60 * time.Second
)

// livePredictor publishes the tracker's live predictions to the page.
type livePredictor struct {
	tracker  *TrajectoryTracker
	ws       WebSocketServer
	interval time.Duration
}

func newLivePredictor(tracker *TrajectoryTracker, ws WebSocketServer) *livePredictor {
	return &livePredictor{tracker: tracker, ws: ws, interval: livePredictionInterval}
}

// run publishes every interval until ctx is done or stop is closed.
func (lp *livePredictor) run(ctx context.Context, stop <-chan struct{}, now func() time.Time) {
	ticker := time.NewTicker(lp.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			lp.broadcast(now())
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// broadcast publishes the predicted state of every aircraft at now, the
// predictions below the confidence floor left out.
func (lp *livePredictor) broadcast(now time.Time) {
	for _, p := range lp.tracker.GetLivePredictionsAt(now) {
		if p.Confidence < minPredictionConfidence {
			continue
		}
		lp.ws.Broadcast(predictedStateMessage(p))
	}
}

// predictedStateMessage is the aircraft_predicted_state message of one
// prediction. Its delta carries the track, not a heading: the receiver's
// true heading (with the wind's drift) arrives with the real updates, and a
// heading equal to the track here made the symbol swing between the two
// (D79).
func predictedStateMessage(p LivePrediction) *websocket.Message {
	return &websocket.Message{
		Type: websocket.MessageTypeAircraftPredictedState,
		Data: map[string]interface{}{
			"hex":          p.Hex,
			"predicted":    true,
			"based_on":     p.BaseObserved.Format(time.RFC3339Nano),
			"predicted_at": p.Timestamp.Format(time.RFC3339Nano),
			"delta": map[string]interface{}{
				"lat":        roundTo(p.Lat, 6),
				"lon":        roundTo(p.Lon, 6),
				"alt_baro":   roundTo(p.Altitude, 0),
				"gs":         roundTo(p.Speed, 0),
				"track":      roundTo(p.Heading, 0),
				"confidence": roundTo(p.Confidence, 3),
				"source":     "predicted",
			},
		},
	}
}

// LivePrediction is an interpolated server-side predicted state for a specific aircraft.
// It is intended for high-frequency websocket updates between real ADS-B polls.
type LivePrediction struct {
	Hex          string    `json:"hex"`
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	Altitude     float64   `json:"altitude"`
	Heading      float64   `json:"heading"`
	Speed        float64   `json:"speed"`
	Confidence   float64   `json:"confidence"`
	Timestamp    time.Time `json:"timestamp"`
	BaseObserved time.Time `json:"base_observed"`
}

// GetLivePredictionsAt returns interpolated live prediction states for all tracked aircraft at time t.
// Real ADS-B updates should always supersede these client-side.
func (tt *TrajectoryTracker) GetLivePredictionsAt(t time.Time) []LivePrediction {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	result := make([]LivePrediction, 0, len(tt.aircraft))
	for hex, at := range tt.aircraft {
		if at == nil || at.Count == 0 {
			continue
		}

		latest := at.Latest()
		if latest == nil || !latest.Valid {
			continue
		}
		if t.Sub(latest.PositionTime()) > maxLivePredictionAge {
			continue
		}

		forecast := at.Prediction.Forecast
		if len(forecast) == 0 {
			continue
		}

		p, ok := interpolateLivePredictionFromForecast(latest, forecast, t)
		if !ok {
			continue
		}

		p.Hex = hex
		p.BaseObserved = latest.Timestamp
		result = append(result, p)
	}

	return result
}

func interpolateLivePredictionFromForecast(latest *TrajectorySnapshot, forecast []PredictionPoint, atTime time.Time) (LivePrediction, bool) {
	if latest == nil || len(forecast) == 0 {
		return LivePrediction{}, false
	}

	first := forecast[0]
	latestHeading := first.Heading
	if latest.TrueHeading != 0 {
		latestHeading = latest.TrueHeading
	} else if latest.MagHeading != 0 {
		latestHeading = latest.MagHeading
	} else if latest.Track != 0 {
		latestHeading = latest.Track
	}

	if atTime.Before(first.Timestamp) {
		start := latest.PositionTime()
		end := first.Timestamp
		if !end.After(start) {
			return LivePrediction{
				Lat:        first.Lat,
				Lon:        first.Lon,
				Altitude:   first.Altitude,
				Heading:    first.Heading,
				Speed:      first.Speed,
				Confidence: first.Confidence,
				Timestamp:  atTime,
			}, true
		}

		alpha := atTime.Sub(start).Seconds() / end.Sub(start).Seconds()
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 1 {
			alpha = 1
		}

		return LivePrediction{
			Lat:        lerpFloat(latest.Lat, first.Lat, alpha),
			Lon:        lerpFloat(latest.Lon, first.Lon, alpha),
			Altitude:   lerpFloat(latest.AltBaro, first.Altitude, alpha),
			Heading:    lerpAngleDeg(latestHeading, first.Heading, alpha),
			Speed:      lerpFloat(latest.GS, first.Speed, alpha),
			Confidence: lerpFloat(0.95, first.Confidence, alpha),
			Timestamp:  atTime,
		}, true
	}

	for i := 0; i < len(forecast)-1; i++ {
		a := forecast[i]
		b := forecast[i+1]
		if atTime.Before(a.Timestamp) || atTime.After(b.Timestamp) {
			continue
		}

		den := b.Timestamp.Sub(a.Timestamp).Seconds()
		if den <= 0 {
			return LivePrediction{}, false
		}
		alpha := atTime.Sub(a.Timestamp).Seconds() / den
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 1 {
			alpha = 1
		}

		return LivePrediction{
			Lat:        lerpFloat(a.Lat, b.Lat, alpha),
			Lon:        lerpFloat(a.Lon, b.Lon, alpha),
			Altitude:   lerpFloat(a.Altitude, b.Altitude, alpha),
			Heading:    lerpAngleDeg(a.Heading, b.Heading, alpha),
			Speed:      lerpFloat(a.Speed, b.Speed, alpha),
			Confidence: lerpFloat(a.Confidence, b.Confidence, alpha),
			Timestamp:  atTime,
		}, true
	}

	last := forecast[len(forecast)-1]
	if atTime.After(last.Timestamp) {
		return LivePrediction{}, false
	}

	return LivePrediction{
		Lat:        last.Lat,
		Lon:        last.Lon,
		Altitude:   last.Altitude,
		Heading:    last.Heading,
		Speed:      last.Speed,
		Confidence: last.Confidence,
		Timestamp:  atTime,
	}, true
}

func lerpFloat(a, b, t float64) float64 {
	return a + (b-a)*t
}

func lerpAngleDeg(a, b, t float64) float64 {
	d := math.Mod((b-a)+540.0, 360.0) - 180.0
	out := a + d*t
	return math.Mod(out+360.0, 360.0)
}
