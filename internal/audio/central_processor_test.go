package audio

import (
	"context"
	"slices"
	"testing"

	"github.com/yegors/co-atc/pkg/logger"
)

func argsFor(t *testing.T, noReconnect bool) []string {
	t.Helper()
	return argsForSource(t, "http://example.invalid/a.mp3", nil, noReconnect)
}

func argsForSource(t *testing.T, url string, inputOptions []string, noReconnect bool) []string {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewCentralAudioProcessor(context.Background(), "x", url,
		CentralProcessorConfig{FFmpegPath: "ffmpeg", SampleRate: 16000, Channels: 1, Format: "s16le",
			FFmpegTimeoutSecs: 10, FFmpegReconnectDelaySecs: 5, NoFFmpegReconnect: noReconnect,
			InputOptions: inputOptions}, log)
	if err != nil {
		t.Fatal(err)
	}
	return p.ffmpegArgs()
}

// Raw PCM over UDP, or a receiver on the sound card, is a source too: what
// ffmpeg needs to read it goes right before -i, in the order given.
func TestInputOptionsComeBeforeTheInput(t *testing.T) {
	opts := []string{"-f", "s16le", "-ar", "48000", "-ac", "1"}
	args := argsForSource(t, "udp://127.0.0.1:7355", opts, false)
	i := slices.Index(args, "-i")
	if i < len(opts) || !slices.Equal(args[i-len(opts):i], opts) {
		t.Errorf("the input options should sit right before -i: %v", args)
	}
	if args[i+1] != "udp://127.0.0.1:7355" {
		t.Errorf("the input follows -i: %v", args)
	}
}

// -timeout and -reconnect are options of ffmpeg's HTTP protocol: on any other
// input ffmpeg stops with "Option reconnect not found" (8.0.1, measured), so
// they are not passed there, whatever the defaults say.
func TestHTTPOptionsStayOffOtherInputs(t *testing.T) {
	httpOnly := []string{"-timeout", "-reconnect", "-reconnect_at_eof", "-reconnect_streamed", "-reconnect_delay_max"}
	for _, url := range []string{"udp://127.0.0.1:7355", ":0", "srt://host:4200", "/tmp/a.wav"} {
		for _, flag := range httpOnly {
			if args := argsForSource(t, url, nil, false); slices.Contains(args, flag) {
				t.Errorf("%s: %s should not be passed: %v", url, flag, args)
			}
		}
	}
	args := argsForSource(t, "HTTPS://example.invalid/a.mp3", nil, false)
	for _, flag := range httpOnly {
		if !slices.Contains(args, flag) {
			t.Errorf("https keeps %s: %v", flag, args)
		}
	}
}

// Upstream's behaviour stays the default: ffmpeg reconnects by itself.
func TestFFmpegReconnectsByDefault(t *testing.T) {
	args := argsFor(t, false)
	for _, flag := range []string{"-reconnect", "-reconnect_at_eof", "-reconnect_streamed", "-reconnect_delay_max"} {
		if !slices.Contains(args, flag) {
			t.Errorf("%s missing from %v", flag, args)
		}
	}
}

// A station's per-channel stream disappears when the station stops listening to
// that channel, and ffmpeg's own reconnection would retry its 404 in a loop.
func TestAStreamThatMayDisappearIsNotRetriedByFFmpeg(t *testing.T) {
	args := argsFor(t, true)
	for _, a := range args {
		if a == "-reconnect" || a == "-reconnect_at_eof" || a == "-reconnect_streamed" || a == "-reconnect_delay_max" {
			t.Errorf("%s should not be passed: %v", a, args)
		}
	}
	if i := slices.Index(args, "-i"); i < 0 || args[i+1] != "http://example.invalid/a.mp3" {
		t.Errorf("the input is still given: %v", args)
	}
}
