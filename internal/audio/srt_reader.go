package audio

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/datarhei/gosrt"
	"github.com/yegors/co-atc/pkg/logger"
)

// SRTReader reads audio data from an SRT stream using native Go SRT library.
// This replaces ffmpeg for SRT streams, providing a cleaner solution with
// fewer processes. The SRT stream is expected to output WAV/PCM s16le data.
type SRTReader struct {
	url            string
	conn           srt.Conn
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *logger.Logger
	mu             sync.Mutex
	isConnected    bool
	lastError      error
	reconnectDelay time.Duration
}

// SRTReaderConfig contains configuration for the SRT reader
type SRTReaderConfig struct {
	ReconnectDelay time.Duration
}

// NewSRTReader creates a new SRT reader for the given URL.
// The URL should be in the format srt://host:port
func NewSRTReader(
	ctx context.Context,
	url string,
	config SRTReaderConfig,
	log *logger.Logger,
) *SRTReader {
	readerCtx, cancel := context.WithCancel(ctx)

	return &SRTReader{
		url:            url,
		ctx:            readerCtx,
		cancel:         cancel,
		logger:         log.Named("srt-reader"),
		reconnectDelay: config.ReconnectDelay,
	}
}

// parseSRTURL parses an SRT URL and returns host:port
// Supports: srt://host:port and srt://host:port?streamid=...
func parseSRTURL(url string) (address string, streamID string, err error) {
	// Remove srt:// prefix
	if !strings.HasPrefix(url, "srt://") {
		return "", "", fmt.Errorf("invalid SRT URL: must start with srt://")
	}

	rest := strings.TrimPrefix(url, "srt://")

	// Split by ? to separate address from query params
	parts := strings.SplitN(rest, "?", 2)
	address = parts[0]

	// Parse query params for streamid
	if len(parts) > 1 {
		params := strings.Split(parts[1], "&")
		for _, param := range params {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) == 2 && strings.ToLower(kv[0]) == "streamid" {
				streamID = kv[1]
				break
			}
		}
	}

	// Validate address has port
	_, _, err = net.SplitHostPort(address)
	if err != nil {
		return "", "", fmt.Errorf("invalid SRT address %q: %w", address, err)
	}

	return address, streamID, nil
}

// Connect establishes connection to the SRT stream
func (r *SRTReader) Connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isConnected {
		return nil
	}

	address, streamID, err := parseSRTURL(r.url)
	if err != nil {
		return fmt.Errorf("failed to parse SRT URL: %w", err)
	}

	r.logger.Info("Connecting to SRT stream",
		String("address", address),
		String("stream_id", streamID))

	// Configure SRT connection for live audio streaming
	// rtl-airband uses live mode with TSBPD enabled
	config := srt.DefaultConfig()
	config.TransmissionType = "live"

	if streamID != "" {
		config.StreamId = streamID
	}

	// Validate config before dialing
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid SRT config: %w", err)
	}

	r.logger.Debug("SRT config",
		String("transmission_type", config.TransmissionType),
		String("stream_id", config.StreamId))

	// Dial the SRT server
	conn, err := srt.Dial("srt", address, config)
	if err != nil {
		r.lastError = err
		return fmt.Errorf("failed to connect to SRT stream: %w", err)
	}

	r.conn = conn
	r.isConnected = true
	r.lastError = nil

	r.logger.Info("Connected to SRT stream successfully",
		String("address", address))

	return nil
}

// Read reads data from the SRT stream
func (r *SRTReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	if !r.isConnected || r.conn == nil {
		r.mu.Unlock()
		return 0, io.EOF
	}
	conn := r.conn
	r.mu.Unlock()

	// Check context
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	n, err = conn.Read(p)
	if err != nil {
		r.mu.Lock()
		r.lastError = err
		r.isConnected = false
		r.mu.Unlock()

		if err == io.EOF {
			r.logger.Warn("SRT stream ended")
		} else {
			r.logger.Error("Error reading from SRT stream", Error(err))
		}
		return n, err
	}

	return n, nil
}

// Close closes the SRT connection
func (r *SRTReader) Close() error {
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn != nil {
		r.logger.Info("Closing SRT connection")
		err := r.conn.Close()
		r.conn = nil
		r.isConnected = false
		return err
	}

	return nil
}

// IsConnected returns whether the reader is currently connected
func (r *SRTReader) IsConnected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isConnected
}

// LastError returns the last error encountered
func (r *SRTReader) LastError() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastError
}
