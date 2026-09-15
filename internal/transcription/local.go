package transcription

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// LocalProcessor transcribes a frequency with a local model instead of the OpenAI
// realtime API, by posting audio to the sidecar described in docs/LOCAL-STT.md.
//
// The realtime API took a continuous stream and told us where speech began and
// ended. A local model does not, so this processor cuts the stream into
// transmissions itself. That turns out to be easy on an SDR feed: RTLSDR-Airband
// emits digital silence between transmissions — measured at -90 dBFS exactly, not
// a noise floor — so squelch openings are unambiguous. On a noisier source the
// maximum-segment guard still bounds how long a segment can grow.
type LocalProcessor struct {
	frequencyID string
	language    string
	audioReader io.ReadCloser
	config      Config
	sink        *transcriptionSink
	client      *http.Client
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *logger.Logger
}

// NewLocalProcessor builds a processor backed by the local STT sidecar.
func NewLocalProcessor(
	ctx context.Context,
	frequencyID string,
	audioReader io.ReadCloser,
	config Config,
	wsServer *websocket.Server,
	storage *sqlite.TranscriptionStorage,
	log *logger.Logger,
	fileLogger *FileLogger,
) (ProcessorInterface, error) {
	if config.Local.ServerURL == "" {
		return nil, fmt.Errorf("transcription.local.server_url is required when backend = \"local\"")
	}

	// The language comes from the frequency catalogue, not from the model. Measured
	// on this station, automatic detection is wrong on 16% of French and 28% of
	// English transmissions, and it fails hardest on clips with no usable speech.
	// The catalogue does not have that problem.
	language := config.FrequencyLanguages[frequencyID]
	if language == "" {
		language = config.Language
	}
	if language == "" {
		language = "en"
	}

	timeout := config.Local.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60 // a zero Timeout means "no limit", which would hang on a dead sidecar
	}

	procCtx, procCancel := context.WithCancel(ctx)
	return &LocalProcessor{
		frequencyID: frequencyID,
		language:    language,
		audioReader: audioReader,
		config:      config,
		sink: &transcriptionSink{
			frequencyID: frequencyID,
			storage:     storage,
			wsServer:    wsServer,
			fileLogger:  fileLogger,
			logger:      log,
		},
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
		ctx:    procCtx,
		cancel: procCancel,
		logger: log.Named("local-xscribe").With(String("frequency_id", frequencyID)),
	}, nil
}

func (p *LocalProcessor) Start() error {
	p.logger.Info("Starting local transcription",
		String("server", p.config.Local.ServerURL),
		String("language", p.language))
	go p.run()
	return nil
}

func (p *LocalProcessor) Stop() error {
	p.logger.Info("Stopping local transcription")
	p.cancel()
	return p.audioReader.Close()
}

// run reads PCM, cuts it into transmissions, and sends each one to the sidecar.
func (p *LocalProcessor) run() {
	rate := p.config.FFmpegSampleRate
	if rate <= 0 {
		rate = 24000
	}
	frame := rate / 50 // 20 ms of s16le mono
	buf := make([]byte, frame*2)

	seg := newSegmenter(rate, p.config.Local)
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		n, err := io.ReadFull(p.audioReader, buf)
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if n == 0 {
					p.logger.Debug("Audio reader drained, waiting")
					time.Sleep(200 * time.Millisecond)
					continue
				}
			} else {
				p.logger.Error("Audio read failed", Error(err))
				return
			}
		}

		if pcm := seg.push(buf[:n]); pcm != nil {
			go p.send(pcm, rate, time.Now())
		}
	}
}

// send posts one transmission and stores whatever comes back.
func (p *LocalProcessor) send(pcm []byte, rate int, at time.Time) {
	req, err := http.NewRequestWithContext(p.ctx, "POST",
		p.config.Local.ServerURL+"/transcribe", bytes.NewReader(pcm))
	if err != nil {
		p.logger.Error("Failed to build sidecar request", Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Sample-Rate", strconv.Itoa(rate))
	req.Header.Set("X-Channels", "1")
	req.Header.Set("X-Language", p.language)
	req.Header.Set("X-Frequency-Id", p.frequencyID)

	resp, err := p.client.Do(req)
	if err != nil {
		if p.ctx.Err() == nil {
			p.logger.Error("Sidecar request failed", Error(err))
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		p.logger.Error("Sidecar returned an error",
			Int("status", resp.StatusCode), String("body", string(body)))
		return
	}

	var out struct {
		Text          string  `json:"text"`
		Language      string  `json:"language"`
		Duration      float64 `json:"duration"`
		SpeechSeconds float64 `json:"speech_seconds"`
		Rejected      string  `json:"rejected"`
		Realtime      float64 `json:"realtime_factor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		p.logger.Error("Failed to decode sidecar response", Error(err))
		return
	}

	// A rejection is a result, not a failure: the sidecar heard no speech, or has
	// no model for this frequency's language. Storing nothing is the correct
	// outcome — a wrong-language transcript reads as a real one downstream.
	if out.Rejected != "" {
		p.logger.Debug("Transmission not transcribed",
			String("reason", out.Rejected),
			String("duration", fmt.Sprintf("%.1fs", out.Duration)))
		return
	}
	if out.Text == "" {
		return
	}

	p.logger.Info("Transcribed",
		String("language", out.Language),
		String("duration", fmt.Sprintf("%.1fs", out.Duration)),
		String("speech", fmt.Sprintf("%.1fs", out.SpeechSeconds)),
		String("realtime", fmt.Sprintf("%.1fx", out.Realtime)),
		String("text", out.Text))

	if err := p.sink.emit(out.Text, at, out.Language); err != nil {
		p.logger.Error("Failed to store transcription", Error(err))
	}
}

// --------------------------------------------------------------------- segmenter

// segmenter cuts a continuous PCM stream into transmissions on squelch activity.
type segmenter struct {
	rate       int
	threshold  float64
	closeAfter int // frames of silence that end a transmission
	minFrames  int
	maxFrames  int
	preroll    int

	speaking bool
	silence  int
	frames   int
	buf      bytes.Buffer
	pre      [][]byte
}

func newSegmenter(rate int, c LocalSTTConfig) *segmenter {
	per := func(ms, def int) int {
		if ms <= 0 {
			ms = def
		}
		return ms / 20 // frames of 20 ms
	}
	th := c.SilenceThreshold
	if th <= 0 {
		th = 0.005 // ≈ -46 dBFS; the stream sits at -90 dBFS when squelch is closed
	}
	return &segmenter{
		rate:       rate,
		threshold:  th,
		closeAfter: per(c.SegmentSilenceMs, 600),
		minFrames:  per(c.SegmentMinMs, 400),
		maxFrames:  per(c.SegmentMaxSeconds*1000, 30000),
		preroll:    per(c.SegmentPrerollMs, 200),
	}
}

// push feeds one 20 ms frame and returns a finished transmission, or nil.
func (s *segmenter) push(frame []byte) []byte {
	loud := rms(frame) > s.threshold

	if !s.speaking {
		// Keep a short pre-roll so the first syllable is not clipped.
		s.pre = append(s.pre, append([]byte(nil), frame...))
		if len(s.pre) > s.preroll {
			s.pre = s.pre[1:]
		}
		if !loud {
			return nil
		}
		s.speaking = true
		s.silence = 0
		s.frames = 0
		s.buf.Reset()
		for _, f := range s.pre {
			s.buf.Write(f)
			s.frames++
		}
		s.pre = s.pre[:0]
	}

	s.buf.Write(frame)
	s.frames++
	if loud {
		s.silence = 0
	} else {
		s.silence++
	}

	if s.silence >= s.closeAfter || s.frames >= s.maxFrames {
		return s.close()
	}
	return nil
}

func (s *segmenter) close() []byte {
	s.speaking = false
	if s.frames-s.silence < s.minFrames {
		s.buf.Reset() // too short to be a transmission
		return nil
	}
	out := append([]byte(nil), s.buf.Bytes()...)
	s.buf.Reset()
	return out
}

func rms(frame []byte) float64 {
	if len(frame) < 2 {
		return 0
	}
	var sum float64
	n := len(frame) / 2
	for i := 0; i < n; i++ {
		v := float64(int16(binary.LittleEndian.Uint16(frame[i*2:]))) / 32768.0
		sum += v * v
	}
	return math.Sqrt(sum / float64(n))
}
