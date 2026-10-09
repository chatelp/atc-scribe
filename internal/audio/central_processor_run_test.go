package audio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

func processorFor(t *testing.T, url string, c CentralProcessorConfig) *CentralAudioProcessor {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	if c.FFmpegPath == "" {
		c.FFmpegPath = "ffmpeg"
	}
	if c.SampleRate == 0 {
		c.SampleRate, c.Channels, c.Format = 16000, 1, "s16le"
	}
	p, err := NewCentralAudioProcessor(context.Background(), "125825", url, c, log)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The whole command line for each kind of source, so that a refactoring that
// moves one option is seen: the global flags, then what only HTTP takes, then
// the source's own input options, the input, and the PCM output on stdout.
func TestTheFFmpegCommandLineForEachKindOfSource(t *testing.T) {
	head := []string{"-loglevel", "error", "-fflags", "nobuffer", "-flags", "low_delay"}
	tail := func(url string) []string {
		return []string{"-i", url, "-f", "s16le", "-acodec", "pcm_s16le", "-ac", "1", "-ar", "16000", "-flush_packets", "1", "pipe:1"}
	}
	reconnect := []string{"-reconnect", "1", "-reconnect_at_eof", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5"}
	cat := func(parts ...[]string) []string { return slices.Concat(parts...) }

	for _, c := range []struct {
		name string
		url  string
		cfg  CentralProcessorConfig
		want []string
	}{
		{"icecast, ffmpeg reconnects", "http://station.invalid/aero.mp3",
			CentralProcessorConfig{FFmpegTimeoutSecs: 10, FFmpegReconnectDelaySecs: 5},
			cat(head, []string{"-timeout", "10000000"}, reconnect, tail("http://station.invalid/aero.mp3"))},
		{"icecast, no timeout", "https://station.invalid/aero.mp3",
			CentralProcessorConfig{FFmpegReconnectDelaySecs: 5},
			cat(head, reconnect, tail("https://station.invalid/aero.mp3"))},
		{"per-channel mount, the processor reconnects", "http://station.invalid/aero-125825.mp3",
			CentralProcessorConfig{FFmpegTimeoutSecs: 10, FFmpegReconnectDelaySecs: 5, NoFFmpegReconnect: true},
			cat(head, []string{"-timeout", "10000000"}, tail("http://station.invalid/aero-125825.mp3"))},
		{"raw PCM over UDP", "udp://127.0.0.1:7355",
			CentralProcessorConfig{FFmpegTimeoutSecs: 10, FFmpegReconnectDelaySecs: 5,
				InputOptions: []string{"-f", "s16le", "-ar", "48000", "-ac", "1"}},
			cat(head, []string{"-f", "s16le", "-ar", "48000", "-ac", "1"}, tail("udp://127.0.0.1:7355"))},
		{"sound card", ":0",
			CentralProcessorConfig{FFmpegTimeoutSecs: 10, FFmpegReconnectDelaySecs: 5,
				InputOptions: []string{"-f", "avfoundation"}},
			cat(head, []string{"-f", "avfoundation"}, tail(":0"))},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := processorFor(t, c.url, c.cfg).ffmpegArgs(); !slices.Equal(got, c.want) {
				t.Errorf("\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func needFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
}

// statuses collects what a processor reports; the callback runs on its own
// goroutine for each change.
type statuses struct {
	mu  sync.Mutex
	got []ConnectionStatus
}

func (s *statuses) record(_ string, st ConnectionStatus, _ string) {
	s.mu.Lock()
	s.got = append(s.got, st)
	s.mu.Unlock()
}

func (s *statuses) count(st ConnectionStatus) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, g := range s.got {
		if g == st {
			n++
		}
	}
	return n
}

func (s *statuses) waitFor(t *testing.T, st ConnectionStatus, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for s.count(st) < n {
		if time.Now().After(deadline) {
			s.mu.Lock()
			defer s.mu.Unlock()
			t.Fatalf("waited for %d %q, got %v", n, st, s.got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A real ffmpeg on a generated tone, as a capture device would be given: the
// transcription's raw reader gets PCM, the browser's reader gets the WAV header
// first, and stopping ends both.
func TestARealFFmpegFeedsBothReaders(t *testing.T) {
	needFFmpeg(t)
	p := processorFor(t, "sine=frequency=440:sample_rate=16000", CentralProcessorConfig{
		ReconnectDelay: time.Hour,
		InputOptions:   []string{"-re", "-f", "lavfi"},
	})
	var st statuses
	p.SetStatusCallback(st.record)
	if state, _, err := p.GetStatus(); state != "stopped" || err != nil {
		t.Errorf("before Start: %q, %v", state, err)
	}
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	st.waitFor(t, StatusConnected, 1)

	raw, err := p.CreateRawReader("transcription")
	if err != nil {
		t.Fatal(err)
	}
	wav, err := p.CreateReader("browser")
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]byte, 3200) // 100 ms
	if _, err := io.ReadFull(raw, pcm); err != nil {
		t.Fatalf("raw PCM: %v", err)
	}
	if bytes.Count(pcm, []byte{0}) == len(pcm) {
		t.Error("a 440 Hz tone should not read as silence")
	}
	head := make([]byte, 44)
	if _, err := io.ReadFull(wav, head); err != nil || !bytes.Equal(head, createWAVHeader(16000, 1)) {
		t.Fatalf("the browser's reader should start with the WAV header: %v", err)
	}
	if _, err := io.ReadFull(wav, make([]byte, 3200)); err != nil {
		t.Fatalf("then PCM: %v", err)
	}

	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	st.waitFor(t, StatusStopped, 1)
	for name, r := range map[string]io.Reader{"raw": raw, "wav": wav} {
		if _, err := r.Read(make([]byte, 64)); !errors.Is(err, io.EOF) {
			t.Errorf("%s reader after Stop: %v, want EOF", name, err)
		}
	}
	if err := p.Stop(); err != nil {
		t.Errorf("a second Stop: %v", err)
	}
}

// A source that ends -- a station that stops publishing -- is reported failed,
// then restarted after the reconnect delay, by the processor itself.
func TestASourceThatEndsIsRestartedAfterTheDelay(t *testing.T) {
	needFFmpeg(t)
	p := processorFor(t, "sine=frequency=440:sample_rate=16000:duration=0.2", CentralProcessorConfig{
		ReconnectDelay: 50 * time.Millisecond,
		InputOptions:   []string{"-f", "lavfi"},
	})
	var st statuses
	p.SetStatusCallback(st.record)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Stop() })

	st.waitFor(t, StatusFailed, 1)
	st.waitFor(t, StatusConnected, 2)
}
