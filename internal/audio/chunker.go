package audio

import (
	"bytes"
	"fmt"
)

// AudioChunker handles chunking of audio data
type AudioChunker struct {
	sampleRate  int
	channels    int
	chunkSizeMs int
	buffer      *bytes.Buffer
	bytesPerMs  int
}

// NewAudioChunker creates a new audio chunker
func NewAudioChunker(sampleRate, channels, chunkSizeMs int) *AudioChunker {
	// Calculate bytes per millisecond
	// For PCM16, each sample is 2 bytes (16 bits)
	bytesPerSample := 2
	bytesPerMs := (sampleRate * channels * bytesPerSample) / 1000

	return &AudioChunker{
		sampleRate:  sampleRate,
		channels:    channels,
		chunkSizeMs: chunkSizeMs,
		buffer:      bytes.NewBuffer(nil),
		bytesPerMs:  bytesPerMs,
	}
}

// ProcessChunk processes an audio chunk and returns base64-encoded chunks
func (c *AudioChunker) ProcessChunk(data []byte) ([][]byte, error) {
	// Add data to buffer
	if _, err := c.buffer.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write to buffer: %w", err)
	}

	// Calculate chunk size in bytes
	chunkSizeBytes := c.chunkSizeMs * c.bytesPerMs

	// Extract chunks
	var chunks [][]byte
	for c.buffer.Len() >= chunkSizeBytes {
		chunk := make([]byte, chunkSizeBytes)
		n, err := c.buffer.Read(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to read from buffer: %w", err)
		}
		if n < chunkSizeBytes {
			// This shouldn't happen, but just in case
			chunk = chunk[:n]
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// Reset resets the buffer
func (c *AudioChunker) Reset() {
	c.buffer.Reset()
}
