package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

type closeRecorder struct {
	io.Reader
	closed bool
}

func (c *closeRecorder) Close() error { c.closed = true; return nil }

// The header a browser reads to play the stream: PCM, 16 bits, the stream's
// rate and channels, and sizes as large as they go since the stream has no end.
func TestTheWAVHeaderDescribesTheStream(t *testing.T) {
	for _, c := range []struct{ rate, channels int }{{16000, 1}, {24000, 1}, {48000, 2}} {
		h := createWAVHeader(c.rate, c.channels)
		if len(h) != 44 {
			t.Fatalf("header of %d bytes, want 44", len(h))
		}
		le := binary.LittleEndian
		for at, want := range map[int]string{0: "RIFF", 8: "WAVE", 12: "fmt ", 36: "data"} {
			if got := string(h[at : at+4]); got != want {
				t.Errorf("%d Hz: %q at %d, want %q", c.rate, got, at, want)
			}
		}
		checks := []struct {
			name      string
			got, want uint32
		}{
			{"riff size", le.Uint32(h[4:]), 0xFFFFFFFF},
			{"fmt size", le.Uint32(h[16:]), 16},
			{"format", uint32(le.Uint16(h[20:])), 1},
			{"channels", uint32(le.Uint16(h[22:])), uint32(c.channels)},
			{"rate", le.Uint32(h[24:]), uint32(c.rate)},
			{"byte rate", le.Uint32(h[28:]), uint32(c.rate * c.channels * 2)},
			{"block align", uint32(le.Uint16(h[32:])), uint32(c.channels * 2)},
			{"bits", uint32(le.Uint16(h[34:])), 16},
			{"data size", le.Uint32(h[40:]), 0xFFFFFFFF - 36},
		}
		for _, k := range checks {
			if k.got != k.want {
				t.Errorf("%d Hz, %d ch: %s = %d, want %d", c.rate, c.channels, k.name, k.got, k.want)
			}
		}
	}
}

// The header comes once, first, followed by the audio in the same read when
// there is room; then the audio alone.
func TestTheHeaderComesOnceThenTheAudio(t *testing.T) {
	audio := pattern(0, 3000)
	wr := NewWAVReader(io.NopCloser(bytes.NewReader(audio)), 16000, 1)
	var got []byte
	buf := make([]byte, 1024)
	for {
		n, err := wr.Read(buf)
		got = append(got, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	want := append(createWAVHeader(16000, 1), audio...)
	if !bytes.Equal(got, want) {
		t.Errorf("read %d bytes, want header + %d of audio", len(got), len(audio))
	}
}

// A buffer smaller than the header is refused without losing the header; one
// exactly its size gets the header alone.
func TestABufferTooSmallForTheHeaderLosesNothing(t *testing.T) {
	audio := pattern(0, 100)
	wr := NewWAVReader(io.NopCloser(bytes.NewReader(audio)), 16000, 1)
	if n, err := wr.Read(make([]byte, 10)); n != 0 || !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("10-byte buffer: %d, %v; want 0, ErrShortBuffer", n, err)
	}
	head := make([]byte, 44)
	if n, err := wr.Read(head); n != 44 || err != nil || !bytes.Equal(head, createWAVHeader(16000, 1)) {
		t.Fatalf("44-byte buffer: %d, %v; want the header alone", n, err)
	}
	rest := make([]byte, 200)
	if n, _ := wr.Read(rest); !bytes.Equal(rest[:n], audio) {
		t.Errorf("then the audio: got %d bytes", n)
	}
}

func TestClosingTheWAVReaderClosesTheStreamReader(t *testing.T) {
	inner := &closeRecorder{Reader: bytes.NewReader(nil)}
	if err := NewWAVReader(inner, 16000, 1).Close(); err != nil || !inner.closed {
		t.Errorf("Close: %v, inner closed %v", err, inner.closed)
	}
}
