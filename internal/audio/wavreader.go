package audio

import (
	"encoding/binary"
	"io"
)

// WAVHeader represents a WAV file header
type WAVHeader struct {
	// RIFF chunk descriptor
	ChunkID   [4]byte // "RIFF"
	ChunkSize uint32  // 4 + (8 + SubChunk1Size) + (8 + SubChunk2Size)
	Format    [4]byte // "WAVE"

	// "fmt " sub-chunk
	Subchunk1ID   [4]byte // "fmt "
	Subchunk1Size uint32  // 16 for PCM
	AudioFormat   uint16  // 1 for PCM
	NumChannels   uint16  // 1 for mono, 2 for stereo
	SampleRate    uint32  // 8000, 44100, etc.
	ByteRate      uint32  // SampleRate * NumChannels * BitsPerSample/8
	BlockAlign    uint16  // NumChannels * BitsPerSample/8
	BitsPerSample uint16  // 8, 16, etc.

	// "data" sub-chunk
	Subchunk2ID   [4]byte // "data"
	Subchunk2Size uint32  // NumSamples * NumChannels * BitsPerSample/8
}

// WAVReader wraps a reader and prepends a WAV header
type WAVReader struct {
	reader     io.ReadCloser
	headerSent bool
	header     []byte
}

// NewWAVReader creates a new WAV reader
func NewWAVReader(reader io.ReadCloser, sampleRate, channels int) *WAVReader {
	// Create WAV header
	header := createWAVHeader(sampleRate, channels)

	return &WAVReader{
		reader:     reader,
		headerSent: false,
		header:     header,
	}
}

// createWAVHeader creates a WAV header with the specified parameters
func createWAVHeader(sampleRate, channels int) []byte {
	bitsPerSample := uint16(16) // 16-bit PCM

	// Calculate derived values
	byteRate := uint32(sampleRate * channels * int(bitsPerSample/8))
	blockAlign := uint16(channels * int(bitsPerSample/8))

	// We don't know the actual data size, so we'll use a very large value
	// This is fine for streaming audio
	dataSize := uint32(0xFFFFFFFF - 36) // Max size minus header size

	// Create header struct
	header := WAVHeader{
		ChunkID:   [4]byte{'R', 'I', 'F', 'F'},
		ChunkSize: 36 + dataSize, // 4 + (8 + 16) + (8 + dataSize)
		Format:    [4]byte{'W', 'A', 'V', 'E'},

		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16, // 16 for PCM
		AudioFormat:   1,  // 1 for PCM
		NumChannels:   uint16(channels),
		SampleRate:    uint32(sampleRate),
		ByteRate:      byteRate,
		BlockAlign:    blockAlign,
		BitsPerSample: bitsPerSample,

		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: dataSize,
	}

	// Convert header to bytes
	headerBytes := make([]byte, 44)

	// RIFF chunk descriptor
	copy(headerBytes[0:4], header.ChunkID[:])
	binary.LittleEndian.PutUint32(headerBytes[4:8], header.ChunkSize)
	copy(headerBytes[8:12], header.Format[:])

	// "fmt " sub-chunk
	copy(headerBytes[12:16], header.Subchunk1ID[:])
	binary.LittleEndian.PutUint32(headerBytes[16:20], header.Subchunk1Size)
	binary.LittleEndian.PutUint16(headerBytes[20:22], header.AudioFormat)
	binary.LittleEndian.PutUint16(headerBytes[22:24], header.NumChannels)
	binary.LittleEndian.PutUint32(headerBytes[24:28], header.SampleRate)
	binary.LittleEndian.PutUint32(headerBytes[28:32], header.ByteRate)
	binary.LittleEndian.PutUint16(headerBytes[32:34], header.BlockAlign)
	binary.LittleEndian.PutUint16(headerBytes[34:36], header.BitsPerSample)

	// "data" sub-chunk
	copy(headerBytes[36:40], header.Subchunk2ID[:])
	binary.LittleEndian.PutUint32(headerBytes[40:44], header.Subchunk2Size)

	return headerBytes
}

// Read reads data from the reader, prepending the WAV header on the first read
func (wr *WAVReader) Read(p []byte) (n int, err error) {
	// If header hasn't been sent yet, send it first
	if !wr.headerSent {
		headerLen := len(wr.header)

		// If buffer is too small for header, return an error
		if len(p) < headerLen {
			return 0, io.ErrShortBuffer
		}

		// Copy header to buffer
		copy(p, wr.header)
		wr.headerSent = true

		// If buffer has room for more data, read from underlying reader
		if len(p) > headerLen {
			m, err := wr.reader.Read(p[headerLen:])
			return headerLen + m, err
		}

		return headerLen, nil
	}

	// Header already sent, just read from underlying reader
	return wr.reader.Read(p)
}

// Close closes the underlying reader
func (wr *WAVReader) Close() error {
	return wr.reader.Close()
}
