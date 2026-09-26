package transcription

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/transcription/phraseology"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// GrammarProcessor is the local counterpart of PostProcessor.
//
// Both occupy the same seam in co-atc: take a stored raw transcription, decide who
// spoke, decide which aircraft it was about, extract any clearance, and write the
// three back. Upstream asks GPT-4o. This one uses the closed-vocabulary grammar in
// phraseology/ and the aircraft the ADS-B receiver can actually see — no key, no
// audio or text leaving the machine, and a decision that can be explained.
//
// It writes into upstream's own tables, with upstream's own vocabulary: speaker_type
// is "ATC" or "PILOT" as the web UI expects, and a clearance is stored under the
// ADS-B callsign so that the existing join in the aircraft API attaches it to the
// right target. Nothing downstream had to change.
//
// Accuracy is measured, not assumed: see docs-fr/17-appariement.md and
// docs-fr/18-nuit-du-15.md. On 1468 recorded transmissions the matcher ran at 75%
// true matches above a shuffled control on CDG approach, and refuses more often
// than it guesses.
type GrammarProcessor struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	transcriptionStorage *sqlite.TranscriptionStorage
	clearanceStorage     *sqlite.ClearanceStorage
	valueStorage         *sqlite.PhraseologyStorage
	wsServer             *websocket.Server

	matcher *phraseology.Matcher
	fleet   FleetProvider

	config   PostProcessingConfig
	interval time.Duration
	logger   *logger.Logger

	// What was just matched on each frequency, used only to break ties between
	// aircraft a transmission already names. See MatchWithContext for why it may
	// never do more than that.
	recentMu sync.Mutex
	recent   map[string][]heard
}

// heard is one aircraft named on a frequency at a moment.
type heard struct {
	at       time.Time
	callsign string
}

// NewGrammarProcessor builds the local post-processor. The airlines file is
// upstream's own assets/airlines.dat; a missing one is fatal rather than silent,
// because without it every spoken operator name is unknown and the matcher would
// quietly degrade to digits alone.
func NewGrammarProcessor(
	ctx context.Context,
	transcriptionStorage *sqlite.TranscriptionStorage,
	clearanceStorage *sqlite.ClearanceStorage,
	wsServer *websocket.Server,
	fleet FleetProvider,
	config PostProcessingConfig,
	log *logger.Logger,
) (*GrammarProcessor, error) {
	matcher, err := phraseology.NewMatcher(config.AirlinesDatPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load airline telephony from %s: %w", config.AirlinesDatPath, err)
	}
	if config.MinDigits > 0 {
		matcher.MinDigits = config.MinDigits
	}

	interval := time.Duration(config.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}

	// Where the extracted levels, headings and squawks are kept, in the same
	// daily database as the transcriptions they came from.
	values := sqlite.NewPhraseologyStorage(sqlite.DBOf(transcriptionStorage))

	procCtx, procCancel := context.WithCancel(ctx)
	return &GrammarProcessor{
		ctx:                  procCtx,
		cancel:               procCancel,
		transcriptionStorage: transcriptionStorage,
		clearanceStorage:     clearanceStorage,
		valueStorage:         values,
		wsServer:             wsServer,
		matcher:              matcher,
		fleet:                fleet,
		config:               config,
		interval:             interval,
		recent:               map[string][]heard{},
		logger:               log.Named("grammar-processor"),
	}, nil
}

// Start begins the annotation loop.
func (p *GrammarProcessor) Start() error {
	if !p.config.Enabled {
		p.logger.Info("Local post-processing is disabled, not starting")
		return nil
	}

	p.logger.Info("Starting local post-processing loop",
		logger.Int("interval_seconds", int(p.interval/time.Second)),
		logger.Int("batch_size", p.config.BatchSize),
		logger.Float64("min_score", p.config.MinScore),
		logger.Int("min_digits", p.matcher.MinDigits),
		logger.Bool("adsb_matching", p.fleet != nil))

	p.backfillValues()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				p.logger.Info("Local post-processing loop stopped")
				return
			case <-ticker.C:
				if err := p.processNextBatch(); err != nil {
					p.logger.Error("Error annotating batch", logger.Error(err))
				}
			}
		}
	}()
	return nil
}

// Stop halts the annotation loop and waits for the current batch.
func (p *GrammarProcessor) Stop() error {
	p.logger.Info("Stopping local post-processing loop")
	p.cancel()
	p.wg.Wait()
	return nil
}

// backfillValues parses transmissions that were annotated before the value table
// existed, so an aircraft selected today shows the facts of transmissions stored
// yesterday.
//
// Bounded, and idempotent by construction: a transcription that already has one
// value row is never returned again, so a restart repairs what is missing and
// touches nothing else. The grammar is deterministic, so re-parsing gives what the
// original pass would have given.
func (p *GrammarProcessor) backfillValues() {
	if p.valueStorage == nil {
		return
	}
	const limit = 5000
	pending, err := p.valueStorage.TranscriptionsWithoutValues(limit)
	if err != nil {
		p.logger.Error("Failed to look for transcriptions without values", logger.Error(err))
		return
	}
	if len(pending) == 0 {
		return
	}

	rows, withValues := 0, 0
	for i := range pending {
		r := pending[i]
		n := p.storeValues(&r, phraseology.Parse(r.Content), r.Callsign)
		if n > 0 {
			withValues++
			rows += n
		}
		// A transmission that yields nothing keeps no marker, so it is examined
		// again on the next start. That is cheap, and the alternative is a column
		// on an upstream table for a question nobody asks.
	}
	p.logger.Info("Backfilled phraseology values",
		logger.Int("examined", len(pending)),
		logger.Int("transmissions_with_values", withValues),
		logger.Int("values_stored", rows))
}

// processNextBatch annotates every transcription that has not been annotated yet.
//
// The fleet is read once per batch rather than once per transcription: within one
// interval the sky has not meaningfully changed, and a single read keeps the
// aircraft list consistent across the batch.
func (p *GrammarProcessor) processNextBatch() error {
	batchSize := p.config.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	records, err := p.transcriptionStorage.GetUnprocessedTranscriptions(batchSize)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed transcriptions: %w", err)
	}
	if len(records) == 0 {
		return nil
	}

	var sky []phraseology.Aircraft
	if p.fleet != nil {
		sky = p.fleet.Fleet()
	}

	for _, record := range records {
		p.annotate(record, sky)
	}
	return nil
}

// annotate fills in one transcription and broadcasts the result.
//
// Every record is marked processed, including the ones the grammar could make
// nothing of. Leaving a record unprocessed would mean retrying it on every tick
// for the life of the database, and the grammar is deterministic: a second attempt
// on the same text yields the same nothing.
func (p *GrammarProcessor) annotate(record *sqlite.TranscriptionRecord, sky []phraseology.Aircraft) {
	result := phraseology.Parse(record.Content)

	processed := strings.TrimSpace(result.Normalized)
	if processed == "" {
		processed = record.Content
	}

	speaker := string(result.Speaker)

	callsign, source := "", ""
	var match phraseology.Match
	// Where the words that named the aircraft sit in the text displayed for the
	// reading that named it -- the processed primary text, or the second reading.
	var evidence [][2]int
	matcher := p.matcher
	if p.config.Rules != nil {
		matcher = p.matcher.WithRules(p.config.Rules())
	}
	// Only the aircraft this frequency can be talking to. With fewer
	// candidates, a flight part missing its last letter ("seven uniform" for
	// 7UE) is worth accepting: measured on 24/09 inside the approach sector.
	if p.config.SectorOf != nil {
		if sector, ok := p.config.SectorOf(record.FrequencyID); ok {
			before := len(sky)
			sky = sector.Within(sky)
			p.logger.Debug("Sky cut to the frequency's sector",
				logger.String("frequency_id", record.FrequencyID),
				logger.Int("aircraft", before), logger.Int("in_sector", len(sky)))
			c := *matcher
			c.PartialFlightScore = 0.6
			matcher = &c
		}
	}
	if len(sky) > 0 {
		ctx := p.recentlyHeard(record.FrequencyID, record.CreatedAt)

		try := func(res phraseology.Result, text string) (phraseology.Match, bool) {
			m, ok := matcher.MatchWithContext(res, sky, ctx)
			if !ok {
				return m, false
			}
			if m.Ambiguous {
				p.logger.Debug("Refusing an ambiguous match",
					logger.Int64("id", record.ID),
					logger.String("text", text),
					logger.String("best", m.Callsign),
					logger.Float64("score", m.Score))
				return m, false
			}
			return m, m.Score >= p.config.MinScore
		}

		primary, okPrimary := try(result, record.Content)

		// The second reading, when the sidecar's French gate opened. Both are
		// tried even when the first succeeds: the match itself is arithmetic on
		// a short list and costs nothing next to the decoding already paid for,
		// and a disagreement recorded today is what lets the rule be remeasured
		// on a bigger corpus tomorrow. Measured on the 15/09 capture (Q29): the
		// two never disagreed inside the gate, 8 agreements out of 8.
		var second phraseology.Match
		var secondResult phraseology.Result
		okSecond := false
		if record.ContentSecond != "" {
			secondResult = phraseology.Parse(record.ContentSecond)
			second, okSecond = try(secondResult, record.ContentSecond)
		}

		switch {
		case okPrimary && okSecond && primary.Callsign != second.Callsign:
			// The primary model wins. It runs on every transmission, so its
			// precision is the one that was measured -- 72% against 65%.
			match, callsign, source = primary, primary.Callsign, "en>fr"
			evidence = result.NormalizedSpans(primary.Words)
			p.logger.Info("The two readings named different aircraft, keeping the primary",
				logger.Int64("id", record.ID),
				logger.String("primary", primary.Callsign),
				logger.String("second", second.Callsign))
		case okPrimary:
			match, callsign, source = primary, primary.Callsign, "en"
			evidence = result.NormalizedSpans(primary.Words)
		case okSecond:
			// The second reading is stored as decoded, so its words are placed in it
			// as it stands.
			match, callsign, source = second, second.Callsign, "fr"
			evidence = secondResult.RawSpans(second.Words)
		}
	}

	if err := p.transcriptionStorage.UpdateMatchedTranscription(
		record.ID, processed, speaker, callsign, source, evidence,
	); err != nil {
		p.logger.Error("Failed to update annotated transcription",
			logger.Int64("id", record.ID), logger.Error(err))
		return
	}

	if callsign != "" {
		p.remember(record.FrequencyID, record.CreatedAt, callsign)
		p.logger.Info("Attached a transmission to an aircraft",
			logger.Int64("id", record.ID),
			logger.String("callsign", callsign),
			logger.String("from", source),
			logger.String("hex", match.Hex),
			logger.Float64("score", match.Score),
			logger.String("why", match.Reason))
	}

	p.storeValues(record, result, callsign)
	p.storeClearances(record, result, callsign)
	p.broadcast(record, processed, speaker, callsign, source, evidence)
}

// recentlyHeard returns the aircraft named on this frequency in the last two
// minutes. Two minutes because an exchange -- instruction, read-back, reply --
// closes well inside that, and anything longer is a different conversation.
func (p *GrammarProcessor) recentlyHeard(frequencyID string, at time.Time) []string {
	const window = 2 * time.Minute

	p.recentMu.Lock()
	defer p.recentMu.Unlock()

	var out []string
	for _, h := range p.recent[frequencyID] {
		if d := at.Sub(h.at); d >= 0 && d <= window {
			out = append(out, h.callsign)
		}
	}
	return out
}

// remember records an aircraft as heard on a frequency.
func (p *GrammarProcessor) remember(frequencyID string, at time.Time, callsign string) {
	const keep = 40

	p.recentMu.Lock()
	defer p.recentMu.Unlock()

	h := append(p.recent[frequencyID], heard{at: at, callsign: callsign})
	if len(h) > keep {
		h = h[len(h)-keep:]
	}
	p.recent[frequencyID] = h
}

// storeValues keeps what the grammar recovered -- levels, headings, runways,
// squawks -- so a transmission can be read as facts and not only as text.
//
// Everything is kept, including values from transmissions no aircraft could be
// found for. A level heard without knowing whose it was still says what the sector
// is working, and discarding it would make the record depend on the matcher.
func (p *GrammarProcessor) storeValues(record *sqlite.TranscriptionRecord, result phraseology.Result, callsign string) int {
	if p.valueStorage == nil || len(result.Values) == 0 {
		return 0
	}
	rows := make([]sqlite.PhraseologyValue, 0, len(result.Values))
	for _, v := range result.Values {
		if v.Role == phraseology.RoleUnknown || v.Role == phraseology.RoleCallsign {
			continue // the callsign has its own column; "unknown" is a non-answer
		}
		rows = append(rows, sqlite.PhraseologyValue{
			TranscriptionID: record.ID,
			Callsign:        callsign,
			Role:            string(v.Role),
			Digits:          v.Digits,
			Text:            v.Text,
			CreatedAt:       record.CreatedAt,
		})
	}
	if err := p.valueStorage.StoreValues(rows); err != nil {
		p.logger.Error("Failed to store phraseology values",
			logger.Int64("id", record.ID), logger.Error(err))
		return 0
	}
	return len(rows)
}

// storeClearances records the clearances a controller issued.
//
// Only matched transmissions produce a clearance row, and only from ATC. The
// aircraft API joins clearances on the ADS-B callsign, so a clearance filed under
// a spoken callsign that no target bears would never be shown to anyone — it would
// only make the table harder to trust. A refusal is recorded in the log instead.
func (p *GrammarProcessor) storeClearances(record *sqlite.TranscriptionRecord, result phraseology.Result, callsign string) {
	if p.clearanceStorage == nil || len(result.Clearances) == 0 {
		return
	}
	if result.Speaker != phraseology.SpeakerATC {
		return
	}
	if callsign == "" {
		p.logger.Debug("Dropping a clearance with no aircraft to attach it to",
			logger.Int64("id", record.ID),
			logger.String("text", record.Content))
		return
	}

	for _, clearance := range result.Clearances {
		stored := &sqlite.ClearanceRecord{
			TranscriptionID: record.ID,
			Callsign:        callsign,
			ClearanceType:   clearance.Type,
			ClearanceText:   clearance.Text,
			Runway:          clearance.Runway,
			Timestamp:       record.CreatedAt,
			Status:          "issued",
			CreatedAt:       time.Now().UTC(),
		}

		id, err := p.clearanceStorage.StoreClearance(stored)
		if err != nil {
			p.logger.Error("Failed to store clearance",
				logger.String("callsign", callsign),
				logger.String("type", clearance.Type),
				logger.Error(err))
			continue
		}
		stored.ID = id

		p.wsServer.Broadcast(&websocket.Message{
			Type: "clearance_issued",
			Data: map[string]interface{}{
				"id":             stored.ID,
				"callsign":       stored.Callsign,
				"clearance_type": stored.ClearanceType,
				"clearance_text": stored.ClearanceText,
				"runway":         stored.Runway,
				"timestamp":      stored.Timestamp,
				"status":         stored.Status,
			},
		})

		p.logger.Info("Stored clearance",
			logger.String("callsign", callsign),
			logger.String("type", clearance.Type),
			logger.String("runway", clearance.Runway),
			logger.Int64("clearance_id", id))
	}
}

// broadcast tells the browser that a transcription now has a speaker and a
// callsign.
//
// "transcription_update" is upstream's own event: www/app.js already handles it and
// patches the three fields into every list it holds. Upstream's post-processor
// stopped sending it — it writes the processed record to a log file instead — so
// the handler is currently dead code. Reviving it is what makes a match visible
// without a page reload.
func (p *GrammarProcessor) broadcast(record *sqlite.TranscriptionRecord, processed, speaker, callsign, source string, evidence [][2]int) {
	p.wsServer.Broadcast(&websocket.Message{
		Type: "transcription_update",
		Data: map[string]interface{}{
			"id":                record.ID,
			"frequency_id":      record.FrequencyID,
			"text":              record.Content,
			"timestamp":         record.CreatedAt,
			"is_complete":       true,
			"is_processed":      true,
			"content_processed": processed,
			"speaker_type":      speaker,
			"callsign":          callsign,
			"callsign_source":   source,
			"callsign_evidence": evidence,
			"content_second":    record.ContentSecond,
		},
	})
}
