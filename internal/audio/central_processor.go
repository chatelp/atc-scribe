package audio

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// Import logger functions
var (
	String = logger.String
	Int    = logger.Int
	Error  = logger.Error
)

// sourceType indicates which audio source backend is being used
type sourceType int

const (
	sourceTypeFFmpeg sourceType = iota // ffmpeg subprocess for HTTP streams
	sourceTypeSRT                      // native SRT for srt:// URLs
)

// CentralAudioProcessor manages audio processing for a frequency
// that can be shared between browser streaming and transcription.
// It automatically uses native SRT for srt:// URLs, or ffmpeg for HTTP streams.
type CentralAudioProcessor struct {
	id                       string
	audioURL                 string
	ffmpegPath               string
	sampleRate               int
	channels                 int
	ffmpegTimeoutSecs        int // FFmpeg connection timeout in seconds
	ffmpegReconnectDelaySecs int // FFmpeg reconnect delay in seconds
	ffmpegCmd                *exec.Cmd
	ffmpegStdout             io.ReadCloser
	srtReader                *SRTReader // Native SRT reader (used instead of ffmpeg for srt://)
	sourceType               sourceType // Which backend is being used
	multiReader              *MultiReader
	ctx                      context.Context
	cancel                   context.CancelFunc
	logger                   *logger.Logger
	mu                       sync.Mutex
	isRunning                bool
	lastError                error
	lastActivity             time.Time
	reconnectTimer           *time.Timer
	monitorTicker            *time.Ticker
	reconnectDelay           time.Duration
	format                   string
	contentType              string
}

// CentralProcessorConfig contains configuration for the central audio processor
type CentralProcessorConfig struct {
	FFmpegPath               string
	SampleRate               int
	Channels                 int
	Format                   string
	ReconnectDelay           time.Duration
	FFmpegTimeoutSecs        int // FFmpeg connection timeout in seconds (0 = no timeout)
	FFmpegReconnectDelaySecs int // FFmpeg reconnect delay in seconds
}

// NewCentralAudioProcessor creates a new central audio processor.
// For srt:// URLs, it uses native Go SRT library instead of ffmpeg.
func NewCentralAudioProcessor(
	ctx context.Context,
	id string,
	audioURL string,
	config CentralProcessorConfig,
	logger *logger.Logger,
) (*CentralAudioProcessor, error) {
	procCtx, procCancel := context.WithCancel(ctx)

	// Create multi-reader for sharing the stream
	multiReader := NewMultiReader(procCtx, logger.Named("multi-reader"))

	// Determine source type based on URL
	srcType := sourceTypeFFmpeg
	if strings.HasPrefix(audioURL, "srt://") {
		srcType = sourceTypeSRT
	}

	return &CentralAudioProcessor{
		id:                       id,
		audioURL:                 audioURL,
		ffmpegPath:               config.FFmpegPath,
		sampleRate:               config.SampleRate,
		channels:                 config.Channels,
		ffmpegTimeoutSecs:        config.FFmpegTimeoutSecs,
		ffmpegReconnectDelaySecs: config.FFmpegReconnectDelaySecs,
		sourceType:               srcType,
		multiReader:              multiReader,
		ctx:                      procCtx,
		cancel:                   procCancel,
		logger:                   logger.Named("central-audio-processor").With(String("id", id)),
		isRunning:                false,
		lastActivity:             time.Now(),
		contentType:              "audio/wav", // We'll be serving WAV format
		format:                   config.Format,
		reconnectDelay:           config.ReconnectDelay,
	}, nil
}

// Start starts the audio processor
func (p *CentralAudioProcessor) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return nil
	}

	p.logger.Info("Starting central audio processor",
		String("url", p.audioURL),
		Int("sample_rate", p.sampleRate),
		Int("channels", p.channels),
		String("source_type", p.sourceTypeString()))

	// Start appropriate source based on URL type
	var err error
	if p.sourceType == sourceTypeSRT {
		err = p.startSRT()
		if err != nil {
			// Fall back to ffmpeg if native SRT fails
			// This can happen with some SRT servers that have incompatible handshake settings
			p.logger.Warn("Native SRT connection failed, falling back to ffmpeg",
				Error(err))
			p.sourceType = sourceTypeFFmpeg
			err = p.startFFmpeg()
			if err != nil {
				return fmt.Errorf("failed to start ffmpeg (fallback): %w", err)
			}
		}
	} else {
		err = p.startFFmpeg()
		if err != nil {
			return fmt.Errorf("failed to start ffmpeg: %w", err)
		}
	}

	// Start monitoring
	p.startMonitoring()

	p.isRunning = true
	return nil
}

// sourceTypeString returns a human-readable string for the source type
func (p *CentralAudioProcessor) sourceTypeString() string {
	switch p.sourceType {
	case sourceTypeSRT:
		return "native-srt"
	case sourceTypeFFmpeg:
		return "ffmpeg"
	default:
		return "unknown"
	}
}

// Stop stops the audio processor
func (p *CentralAudioProcessor) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return nil
	}

	p.logger.Info("Stopping central audio processor",
		String("source_type", p.sourceTypeString()))

	// Stop monitoring
	if p.monitorTicker != nil {
		p.monitorTicker.Stop()
		p.monitorTicker = nil
	}

	// Cancel context to stop all operations
	p.cancel()

	// Stop the appropriate source
	if p.sourceType == sourceTypeSRT {
		p.stopSRT()
	} else {
		p.stopFFmpeg()
	}

	// Close multi-reader
	p.multiReader.Close()

	p.isRunning = false
	return nil
}

// startSRT starts the native SRT reader
func (p *CentralAudioProcessor) startSRT() error {
	p.logger.Debug("Starting native SRT reader",
		String("url", p.audioURL))

	// Create SRT reader
	p.srtReader = NewSRTReader(p.ctx, p.audioURL, SRTReaderConfig{
		ReconnectDelay: p.reconnectDelay,
	}, p.logger)

	// Connect to SRT stream
	if err := p.srtReader.Connect(); err != nil {
		return fmt.Errorf("failed to connect to SRT stream: %w", err)
	}

	// Start copying data from SRT to multi-reader
	go p.processSRTOutput()

	return nil
}

// stopSRT stops the native SRT reader
func (p *CentralAudioProcessor) stopSRT() {
	if p.srtReader != nil {
		p.logger.Info("Stopping SRT reader")
		_ = p.srtReader.Close()
		p.srtReader = nil
	}

	if p.reconnectTimer != nil {
		p.reconnectTimer.Stop()
		p.reconnectTimer = nil
	}
}

// processSRTOutput reads from SRT and writes to multi-reader
func (p *CentralAudioProcessor) processSRTOutput() {
	p.logger.Info("Starting to process SRT output")

	buffer := make([]byte, 4096)
	bytesProcessed := 0
	lastLogTime := time.Now()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Context canceled, stopping SRT output processing",
				Int("total_bytes_processed", bytesProcessed))
			return
		default:
			n, err := p.srtReader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					p.logger.Warn("SRT stream ended unexpectedly",
						Int("total_bytes_processed", bytesProcessed),
						String("duration_since_start", time.Since(lastLogTime).String()))
				} else {
					p.logger.Error("Error reading from SRT", Error(err),
						Int("total_bytes_processed", bytesProcessed),
						String("duration_since_start", time.Since(lastLogTime).String()))
					p.lastError = err
				}

				// Attempt to restart SRT after a delay
				p.mu.Lock()
				if p.isRunning && p.reconnectTimer == nil {
					p.logger.Warn("Scheduling SRT restart due to read error",
						String("error_type", fmt.Sprintf("%T", err)),
						String("error_message", err.Error()))
					p.reconnectTimer = time.AfterFunc(p.reconnectDelay, func() {
						p.mu.Lock()
						defer p.mu.Unlock()

						p.reconnectTimer = nil
						if p.isRunning {
							p.logger.Info("Executing scheduled SRT restart")
							p.stopSRT()
							if err := p.startSRT(); err != nil {
								p.logger.Error("Failed to restart SRT", Error(err))
							} else {
								p.logger.Info("SRT restarted successfully")
							}
						}
					})
				}
				p.mu.Unlock()
				return
			}

			if n > 0 {
				bytesProcessed += n
				p.lastActivity = time.Now()

				// Log progress every 30 seconds
				if time.Since(lastLogTime) > 30*time.Second {
					p.logger.Debug("SRT processing progress",
						Int("bytes_processed", bytesProcessed),
						Int("bytes_this_read", n),
						String("duration", time.Since(lastLogTime).String()))
					lastLogTime = time.Now()
				}

				// Write to multi-reader
				if _, err := p.multiReader.Write(buffer[:n]); err != nil {
					p.logger.Error("Error writing to multi-reader", Error(err),
						Int("bytes_processed_before_error", bytesProcessed))
					return
				}
			}
		}
	}
}

// startFFmpeg starts the ffmpeg process
func (p *CentralAudioProcessor) startFFmpeg() error {
	p.logger.Debug("Starting ffmpeg process",
		String("path", p.ffmpegPath),
		String("url", p.audioURL))

	// Create FFmpeg command with different options based on stream type
	var args []string

	// Check if this is an SRT stream
	if strings.HasPrefix(p.audioURL, "srt://") {
		// SRT stream configuration - optimized for low latency
		args = []string{
			"-loglevel", "error", // Minimal logging
			"-fflags", "nobuffer", // Disable input buffering
			"-flags", "low_delay", // Enable low delay mode
			"-i", p.audioURL, // Input SRT URL
			"-f", p.format, // Output format (should be s16le for raw PCM)
			"-acodec", "pcm_s16le", // Audio codec
			"-ac", fmt.Sprintf("%d", p.channels), // Channels
			"-ar", fmt.Sprintf("%d", p.sampleRate), // Sample rate
			"-flush_packets", "1", // Flush packets immediately
			"pipe:1", // Output to stdout
		}
	} else {
		// HTTP stream configuration - optimized for low latency with reconnection
		args = []string{
			"-loglevel", "error", // Minimal logging
			"-fflags", "nobuffer", // Disable input buffering
			"-flags", "low_delay", // Enable low delay mode
		}

		// Add timeout if configured (convert seconds to microseconds)
		if p.ffmpegTimeoutSecs > 0 {
			timeoutMicros := p.ffmpegTimeoutSecs * 1000000
			args = append(args, "-timeout", fmt.Sprintf("%d", timeoutMicros))
		}

		// Add reconnection settings
		args = append(args,
			"-reconnect", "1", // Enable reconnection
			"-reconnect_at_eof", "1", // Reconnect at end of file
			"-reconnect_streamed", "1", // Reconnect for streamed inputs
			"-reconnect_delay_max", fmt.Sprintf("%d", p.ffmpegReconnectDelaySecs), // Configurable reconnect delay
			"-i", p.audioURL, // Input URL
			"-f", p.format, // Output format (should be s16le for raw PCM)
			"-acodec", "pcm_s16le", // Audio codec
			"-ac", fmt.Sprintf("%d", p.channels), // Channels
			"-ar", fmt.Sprintf("%d", p.sampleRate), // Sample rate
			"-flush_packets", "1", // Flush packets immediately
			"pipe:1", // Output to stdout
		)
	}

	// Create ffmpeg command with enhanced arguments
	p.ffmpegCmd = exec.CommandContext(p.ctx, p.ffmpegPath, args...)

	// Get stdout pipe
	var err error
	p.ffmpegStdout, err = p.ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start ffmpeg
	if err := p.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Start copying data from ffmpeg to multi-reader
	go p.processFFmpegOutput()

	return nil
}

// stopFFmpeg stops the ffmpeg process
func (p *CentralAudioProcessor) stopFFmpeg() {
	if p.ffmpegCmd != nil && p.ffmpegCmd.Process != nil {
		p.logger.Info("Stopping ffmpeg process")

		// Try to kill the process, but don't log errors during shutdown
		// These errors are expected as ffmpeg may already be terminated
		_ = p.ffmpegCmd.Process.Kill()

		// Wait for the process to exit, but don't log errors
		// The exit status might be non-zero or the process might already be gone
		_ = p.ffmpegCmd.Wait()
	}

	if p.reconnectTimer != nil {
		p.reconnectTimer.Stop()
		p.reconnectTimer = nil
	}
}

// processFFmpegOutput processes the output from ffmpeg
func (p *CentralAudioProcessor) processFFmpegOutput() {
	p.logger.Info("Starting to process ffmpeg output")

	// Create buffer for reading
	buffer := make([]byte, 4096)
	bytesProcessed := 0
	lastLogTime := time.Now()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Context canceled, stopping ffmpeg output processing",
				Int("total_bytes_processed", bytesProcessed))
			return
		default:
			// Read from ffmpeg
			n, err := p.ffmpegStdout.Read(buffer)
			if err != nil {
				if err == io.EOF {
					p.logger.Warn("FFmpeg output ended unexpectedly",
						Int("total_bytes_processed", bytesProcessed),
						String("duration_since_start", time.Since(lastLogTime).String()))
				} else {
					p.logger.Error("Error reading from ffmpeg", Error(err),
						Int("total_bytes_processed", bytesProcessed),
						String("duration_since_start", time.Since(lastLogTime).String()))
					p.lastError = err
				}

				// Attempt to restart ffmpeg after a delay
				p.mu.Lock()
				if p.isRunning && p.reconnectTimer == nil {
					p.logger.Warn("Scheduling ffmpeg restart due to read error",
						String("error_type", fmt.Sprintf("%T", err)),
						String("error_message", err.Error()))
					p.reconnectTimer = time.AfterFunc(p.reconnectDelay, func() {
						p.mu.Lock()
						defer p.mu.Unlock()

						p.reconnectTimer = nil
						if p.isRunning {
							p.logger.Info("Executing scheduled ffmpeg restart")
							p.stopFFmpeg()
							if err := p.startFFmpeg(); err != nil {
								p.logger.Error("Failed to restart ffmpeg", Error(err))
							} else {
								p.logger.Info("FFmpeg restarted successfully")
							}
						}
					})
				}
				p.mu.Unlock()
				return
			}

			if n > 0 {
				bytesProcessed += n
				// Update last activity time
				p.lastActivity = time.Now()

				// Log progress every 30 seconds
				if time.Since(lastLogTime) > 30*time.Second {
					p.logger.Debug("FFmpeg processing progress",
						Int("bytes_processed", bytesProcessed),
						Int("bytes_this_read", n),
						String("duration", time.Since(lastLogTime).String()))
					lastLogTime = time.Now()
				}

				// Write to multi-reader
				if _, err := p.multiReader.Write(buffer[:n]); err != nil {
					p.logger.Error("Error writing to multi-reader", Error(err),
						Int("bytes_processed_before_error", bytesProcessed))
					return
				}
			}
		}
	}
}

// startMonitoring starts monitoring the audio source (ffmpeg or SRT)
func (p *CentralAudioProcessor) startMonitoring() {
	p.monitorTicker = time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-p.monitorTicker.C:
				p.mu.Lock()
				if p.sourceType == sourceTypeSRT {
					// Monitor SRT connection
					if p.isRunning && p.srtReader != nil && !p.srtReader.IsConnected() {
						p.logger.Warn("SRT connection lost")

						if p.isRunning && p.reconnectTimer == nil {
							p.logger.Info("Restarting SRT after connection loss")
							p.stopSRT()
							if err := p.startSRT(); err != nil {
								p.logger.Error("Failed to restart SRT", Error(err))
							}
						}
					}
				} else {
					// Monitor ffmpeg process
					if p.isRunning && p.ffmpegCmd != nil && p.ffmpegCmd.ProcessState != nil {
						p.logger.Warn("FFmpeg process has exited unexpectedly")

						if p.isRunning && p.reconnectTimer == nil {
							p.logger.Info("Restarting ffmpeg after unexpected exit")
							p.stopFFmpeg()
							if err := p.startFFmpeg(); err != nil {
								p.logger.Error("Failed to restart ffmpeg", Error(err))
							}
						}
					}
				}
				p.mu.Unlock()
			}
		}
	}()
}

// CreateReader creates a new reader for the audio stream
func (p *CentralAudioProcessor) CreateReader(id string) (io.ReadCloser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		var err error
		if p.sourceType == sourceTypeSRT {
			err = p.startSRT()
			if err != nil {
				// Fall back to ffmpeg if native SRT fails
				p.logger.Warn("Native SRT connection failed, falling back to ffmpeg",
					Error(err))
				p.sourceType = sourceTypeFFmpeg
				err = p.startFFmpeg()
			}
		} else {
			err = p.startFFmpeg()
		}
		if err != nil {
			return nil, fmt.Errorf("failed to start processor: %w", err)
		}
		p.isRunning = true
	}

	// Create a reader with WAV header
	reader := p.multiReader.CreateReader(id)
	return NewWAVReader(reader, p.sampleRate, p.channels), nil
}

// RemoveReader removes a reader
func (p *CentralAudioProcessor) RemoveReader(id string) {
	p.multiReader.RemoveReader(id)
}

// GetStatus returns the status of the processor
func (p *CentralAudioProcessor) GetStatus() (string, time.Time, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return "stopped", p.lastActivity, nil
	}

	if p.lastError != nil {
		return "error", p.lastActivity, p.lastError
	}

	return "running", p.lastActivity, nil
}

// GetContentType returns the content type of the audio stream
func (p *CentralAudioProcessor) GetContentType() string {
	return p.contentType
}

// GetFormat returns the format of the audio stream
func (p *CentralAudioProcessor) GetFormat() string {
	return p.format
}
