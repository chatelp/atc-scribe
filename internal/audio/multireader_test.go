package audio

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/yegors/co-atc/pkg/logger"
)

func newTestMultiReader(t *testing.T) *MultiReader {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatal(err)
	}
	mr := NewMultiReader(context.Background(), log)
	t.Cleanup(func() { mr.Close() })
	return mr
}

// pattern is recognisable data: byte i is i mod 251, so any loss, repetition or
// reordering changes what is read.
func pattern(from, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte((from + i) % 251)
	}
	return b
}

func readAll(t *testing.T, r io.Reader, n int) []byte {
	t.Helper()
	out := make([]byte, 0, n)
	buf := make([]byte, 960) // 20 ms at 24 kHz, as the transcription reads
	for len(out) < n {
		k, err := r.Read(buf)
		if err != nil {
			t.Fatalf("after %d bytes: %v", len(out), err)
		}
		out = append(out, buf[:k]...)
	}
	return out
}

// An Icecast burst decoded at once: a minute of audio written before the
// reader reads any of it. Upstream's 64 KB ring kept the last 1.3 s of it.
func TestABurstFasterThanTheReaderIsReadWhole(t *testing.T) {
	mr := newTestMultiReader(t)
	r := mr.CreateReader("transcription")
	const minute = 60 * 48000 // 60 s of 24 kHz mono s16le
	for off := 0; off < minute; off += 4096 {
		n := min(4096, minute-off)
		mr.Write(pattern(off, n))
	}
	if got := readAll(t, r, minute); !bytes.Equal(got, pattern(0, minute)) {
		t.Fatal("a burst that fits the buffer should be read whole and in order")
	}
}

// A reader stuck for longer than the buffer holds loses the oldest audio, not
// the order of what follows.
func TestAReaderLeftBehindGetsTheNewestAudioInOrder(t *testing.T) {
	mr := newTestMultiReader(t)
	r := mr.CreateReader("slow")
	total := 2*multiReaderBytes + 1000
	for off := 0; off < total; off += 4096 {
		mr.Write(pattern(off, min(4096, total-off)))
	}
	got := readAll(t, r, multiReaderBytes)
	if want := pattern(total-multiReaderBytes, multiReaderBytes); !bytes.Equal(got, want) {
		t.Fatal("a reader left behind should get the last buffer's worth, in order")
	}
}

// A reader that joins late starts live, as upstream's did: a browser that
// starts listening does not want the last minute first.
func TestALateReaderStartsLive(t *testing.T) {
	mr := newTestMultiReader(t)
	mr.Write(pattern(0, 10000))
	r := mr.CreateReader("browser")
	mr.Write(pattern(10000, 500))
	if got := readAll(t, r, 500); !bytes.Equal(got, pattern(10000, 500)) {
		t.Fatal("a new reader should only see what is written after it joined")
	}
}
