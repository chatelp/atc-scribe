package adsb

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// ─── Runway-In-Use Detection ─────────────────────────────────────────────────
//
// RunwayInUseTracker observes aircraft movements (approaches, landings,
// departures) to determine which runway ends are currently active. It maintains
// a rolling time window of weighted evidence events and produces a probability
// distribution over runway ends.
//
// The primary consumer is ruleApproach: when we have enough data to be
// confident about the active runway, approaches to non-active runways (e.g.
// perpendicular cross-runways during base turns) are suppressed.
//
// Thread-safe: the fetch goroutine calls RecordEvent and IsActiveRunway from
// the same goroutine, but the tracker may also be queried from API handlers.

// RunwayEventType classifies evidence events by their source.
type RunwayEventType int

const (
	RunwayEventApproach RunwayEventType = iota // Aircraft entered APP on this runway end
	RunwayEventLanding                         // Aircraft touched down (T/D)
	RunwayEventClimb                           // Aircraft climbed out (CLB) on this runway end
)

func (t RunwayEventType) String() string {
	switch t {
	case RunwayEventApproach:
		return "approach"
	case RunwayEventLanding:
		return "landing"
	case RunwayEventClimb:
		return "climb"
	default:
		return "unknown"
	}
}

// RunwayEvent records a single piece of evidence that a runway end is in use.
type RunwayEvent struct {
	RunwayEnd string          // "05-23/05" format (matches RunwayApproachInfo.RunwayID)
	Type      RunwayEventType
	Hex       string // Aircraft that generated this event
	Timestamp time.Time
}

// RunwayScore holds the computed score and probability for a runway end.
type RunwayScore struct {
	RunwayEnd   string  `json:"runway_end"`
	Score       float64 `json:"score"`
	Probability float64 `json:"probability"` // normalized 0-1
	EventCount  int     `json:"event_count"`
}

// RunwayInUseTracker maintains a rolling window of runway usage evidence
// and computes which runway ends are currently active.
type RunwayInUseTracker struct {
	mu             sync.RWMutex
	events         []RunwayEvent
	windowDuration time.Duration
	weights        [3]float64 // indexed by RunwayEventType
	decayRate      float64    // per-minute exponential decay
	logger         *logger.Logger

	// Cached state from last recompute
	scores        []RunwayScore // sorted descending by score
	activeSet     map[string]bool
	lastActiveEnd string    // for change detection
	lastLogTime   time.Time // throttle periodic Info log

	// Persisted state — retained when all events expire from the window so
	// that the last known active runway is shown instead of N/A during
	// low-traffic periods.
	everHadData bool
}

const (
	// Minimum probability (relative) for a runway to be considered "active".
	// Handles parallel runway operations (e.g. two parallel runways both at ~40%).
	activeMinProbability = 0.15

	// Minimum interval between periodic score logs.
	logThrottleInterval = 60 * time.Second
)

// NewRunwayInUseTracker creates a tracker with the given scoring parameters.
func NewRunwayInUseTracker(
	windowMinutes int,
	approachWeight, landingWeight, climbWeight, decayRate float64,
	log *logger.Logger,
) *RunwayInUseTracker {
	rt := &RunwayInUseTracker{
		windowDuration: time.Duration(windowMinutes) * time.Minute,
		weights:        [3]float64{approachWeight, landingWeight, climbWeight},
		decayRate:      decayRate,
		logger:         log.Named("runway-use"),
		activeSet:      make(map[string]bool),
	}
	rt.logger.Info("Runway-in-use tracker started",
		logger.Int("window_minutes", windowMinutes),
		logger.Float64("approach_weight", approachWeight),
		logger.Float64("landing_weight", landingWeight),
		logger.Float64("climb_weight", climbWeight),
		logger.Float64("decay_rate", decayRate),
	)
	return rt
}

// RecordEvent records evidence of runway usage and recomputes scores.
func (rt *RunwayInUseTracker) RecordEvent(runwayID string, eventType RunwayEventType, hex string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now().UTC()
	rt.events = append(rt.events, RunwayEvent{
		RunwayEnd: runwayID,
		Type:      eventType,
		Hex:       hex,
		Timestamp: now,
	})

	rt.logger.Debug("Runway event recorded",
		logger.String("runway", runwayID),
		logger.String("type", eventType.String()),
		logger.String("hex", hex),
	)

	rt.recompute(now)
}

// IsActiveRunway checks whether a runway end is in the current active set.
// Returns true during startup grace (never had data) so that all runways are
// accepted until enough traffic has been observed. Once data has been seen,
// uses the active set (which persists across inactivity periods).
func (rt *RunwayInUseTracker) IsActiveRunway(runwayID string) bool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Startup grace: never had any data → accept all runways
	if !rt.everHadData {
		return true
	}
	return rt.activeSet[runwayID]
}

// HasData returns true if there are any events in the current window.
func (rt *RunwayInUseTracker) HasData() bool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.events) > 0
}

// GetTopScores returns the top N runway scores, sorted by score descending.
func (rt *RunwayInUseTracker) GetTopScores(n int) []RunwayScore {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if n > len(rt.scores) {
		n = len(rt.scores)
	}
	result := make([]RunwayScore, n)
	copy(result, rt.scores[:n])
	return result
}

// recompute prunes old events, recalculates time-decayed scores, and updates
// the active set. Must be called with rt.mu held for writing.
func (rt *RunwayInUseTracker) recompute(now time.Time) {
	// ── Prune events outside the window ──
	cutoff := now.Add(-rt.windowDuration)
	writeIdx := 0
	for _, e := range rt.events {
		if e.Timestamp.After(cutoff) {
			rt.events[writeIdx] = e
			writeIdx++
		}
	}
	rt.events = rt.events[:writeIdx]

	// ── Compute time-decayed weighted scores per runway end ──
	type accumulator struct {
		score float64
		count int
	}
	scoreMap := make(map[string]*accumulator)

	for _, e := range rt.events {
		minutesAgo := now.Sub(e.Timestamp).Minutes()
		weight := rt.weights[e.Type] * math.Pow(rt.decayRate, minutesAgo)

		acc, ok := scoreMap[e.RunwayEnd]
		if !ok {
			acc = &accumulator{}
			scoreMap[e.RunwayEnd] = acc
		}
		acc.score += weight
		acc.count++
	}

	// ── Build sorted score list ──
	scores := make([]RunwayScore, 0, len(scoreMap))
	totalScore := 0.0
	for end, acc := range scoreMap {
		totalScore += acc.score
		scores = append(scores, RunwayScore{
			RunwayEnd:  end,
			Score:      acc.score,
			EventCount: acc.count,
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// ── Normalize to probabilities ──
	if totalScore > 0 {
		for i := range scores {
			scores[i].Probability = scores[i].Score / totalScore
		}
	}

	// If we have live data, update scores and mark that we've seen data.
	// If all events expired (low traffic), keep the last known scores/activeSet
	// so the UI shows the last active runway instead of N/A.
	if len(scores) > 0 {
		rt.scores = scores
		rt.everHadData = true

		// ── Build active set (probability >= threshold) ──
		rt.activeSet = make(map[string]bool, len(scores))
		for _, s := range scores {
			if s.Probability >= activeMinProbability {
				rt.activeSet[s.RunwayEnd] = true
			}
		}
	}
	// else: keep previous rt.scores and rt.activeSet intact

	// ── Logging ──
	newActiveEnd := ""
	if len(rt.scores) > 0 {
		newActiveEnd = rt.scores[0].RunwayEnd
	}

	// Log on active runway change
	if newActiveEnd != rt.lastActiveEnd && newActiveEnd != "" {
		if rt.lastActiveEnd != "" {
			rt.logger.Info("Active runway changed",
				logger.String("previous", formatRunwayEnd(rt.lastActiveEnd)),
				logger.String("current", formatRunwayEnd(newActiveEnd)),
				logger.String("scores", formatScores(rt.scores)),
			)
		} else {
			rt.logger.Info("Initial active runway detected",
				logger.String("runway", formatRunwayEnd(newActiveEnd)),
				logger.String("scores", formatScores(rt.scores)),
			)
		}
		rt.lastActiveEnd = newActiveEnd
	}

	// Periodic score summary (throttled)
	if now.Sub(rt.lastLogTime) >= logThrottleInterval {
		if len(rt.scores) > 0 {
			n := 3
			if n > len(rt.scores) {
				n = len(rt.scores)
			}
			rt.logger.Info("Runway in use",
				logger.String("scores", formatScores(rt.scores[:n])),
				logger.Int("total_events", len(rt.events)),
			)
		}
		rt.lastLogTime = now
	}
}

// formatRunwayEnd extracts a human-readable runway name from "05-23/05" format.
// Returns just the end identifier (e.g. "05") with the pair for context.
func formatRunwayEnd(runwayID string) string {
	// RunwayID format: "pairKey/endIdent" e.g. "05-23/05"
	parts := strings.SplitN(runwayID, "/", 2)
	if len(parts) == 2 {
		return fmt.Sprintf("RWY %s (%s)", parts[1], parts[0])
	}
	return runwayID
}

// formatScores produces a compact log-friendly string of runway scores.
func formatScores(scores []RunwayScore) string {
	var b strings.Builder
	for i, s := range scores {
		if i > 0 {
			b.WriteString(" | ")
		}
		fmt.Fprintf(&b, "%s: %.1f (%.0f%%, %d events)",
			formatRunwayEnd(s.RunwayEnd), s.Score, s.Probability*100, s.EventCount)
	}
	return b.String()
}
