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

// CentralAudioProcessor manages a single ffmpeg process for a frequency
// that can be shared between browser streaming and transcription
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

// NewCentralAudioProcessor creates a new central audio processor
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

	return &CentralAudioProcessor{
		id:                       id,
		audioURL:                 audioURL,
		ffmpegPath:               config.FFmpegPath,
		sampleRate:               config.SampleRate,
		channels:                 config.Channels,
		ffmpegTimeoutSecs:        config.FFmpegTimeoutSecs,
		ffmpegReconnectDelaySecs: config.FFmpegReconnectDelaySecs,
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
		Int("channels", p.channels))

	// Start the ffmpeg process
	if err := p.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Start monitoring the ffmpeg process
	p.startMonitoring()

	p.isRunning = true
	return nil
}

// Stop stops the audio processor
func (p *CentralAudioProcessor) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return nil
	}

	p.logger.Info("Stopping central audio processor")

	// Stop monitoring
	if p.monitorTicker != nil {
		p.monitorTicker.Stop()
		p.monitorTicker = nil
	}

	// Cancel context to stop all operations
	p.cancel()

	// Stop ffmpeg
	p.stopFFmpeg()

	// Close multi-reader
	p.multiReader.Close()

	p.isRunning = false
	return nil
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

// startMonitoring starts monitoring the ffmpeg process
func (p *CentralAudioProcessor) startMonitoring() {
	p.monitorTicker = time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-p.monitorTicker.C:
				p.mu.Lock()
				if p.isRunning && p.ffmpegCmd != nil && p.ffmpegCmd.ProcessState != nil {
					// Process has exited
					p.logger.Warn("FFmpeg process has exited unexpectedly")

					// Only restart if we're still running
					if p.isRunning && p.reconnectTimer == nil {
						p.logger.Info("Restarting ffmpeg after unexpected exit")
						p.stopFFmpeg()
						if err := p.startFFmpeg(); err != nil {
							p.logger.Error("Failed to restart ffmpeg", Error(err))
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
		if err := p.startFFmpeg(); err != nil {
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
