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
	wsServer             *websocket.Server

	matcher *phraseology.Matcher
	fleet   FleetProvider

	config   PostProcessingConfig
	interval time.Duration
	logger   *logger.Logger
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

	procCtx, procCancel := context.WithCancel(ctx)
	return &GrammarProcessor{
		ctx:                  procCtx,
		cancel:               procCancel,
		transcriptionStorage: transcriptionStorage,
		clearanceStorage:     clearanceStorage,
		wsServer:             wsServer,
		matcher:              matcher,
		fleet:                fleet,
		config:               config,
		interval:             interval,
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

	callsign := ""
	var match phraseology.Match
	if len(sky) > 0 {
		if m, ok := p.matcher.Match(result, sky); ok && !m.Ambiguous && m.Score >= p.config.MinScore {
			match = m
			callsign = m.Callsign
		} else if ok && m.Ambiguous {
			p.logger.Debug("Refusing an ambiguous match",
				logger.Int64("id", record.ID),
				logger.String("text", record.Content),
				logger.String("best", m.Callsign),
				logger.Float64("score", m.Score))
		}
	}

	if err := p.transcriptionStorage.UpdateProcessedTranscription(
		record.ID, processed, speaker, callsign,
	); err != nil {
		p.logger.Error("Failed to update annotated transcription",
			logger.Int64("id", record.ID), logger.Error(err))
		return
	}

	if callsign != "" {
		p.logger.Info("Attached a transmission to an aircraft",
			logger.Int64("id", record.ID),
			logger.String("callsign", callsign),
			logger.String("hex", match.Hex),
			logger.Float64("score", match.Score),
			logger.String("why", match.Reason))
	}

	p.storeClearances(record, result, callsign)
	p.broadcast(record, processed, speaker, callsign)
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
func (p *GrammarProcessor) broadcast(record *sqlite.TranscriptionRecord, processed, speaker, callsign string) {
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
		},
	})
}
