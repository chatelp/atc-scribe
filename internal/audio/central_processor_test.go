package audio

import (
	"context"
	"slices"
	"testing"

	"github.com/yegors/co-atc/pkg/logger"
)

func argsFor(t *testing.T, noReconnect bool) []string {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewCentralAudioProcessor(context.Background(), "x", "http://example.invalid/a.mp3",
		CentralProcessorConfig{FFmpegPath: "ffmpeg", SampleRate: 16000, Channels: 1, Format: "s16le",
			FFmpegReconnectDelaySecs: 5, NoFFmpegReconnect: noReconnect}, log)
	if err != nil {
		t.Fatal(err)
	}
	return p.ffmpegArgs()
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
