package transcription

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// FileLogger handles append-only logging of transcriptions to files
type FileLogger struct {
	baseDir string
	logger  *logger.Logger
	mu      sync.Mutex
	files   map[string]*os.File // Cache of open file handles
}

// NewFileLogger creates a new FileLogger
// If baseDir is empty, the logger is disabled and all operations are no-ops
func NewFileLogger(baseDir string, logger *logger.Logger) (*FileLogger, error) {
	fl := &FileLogger{
		baseDir: baseDir,
		logger:  logger,
		files:   make(map[string]*os.File),
	}

	// If baseDir is empty, logging is disabled
	if baseDir == "" {
		return fl, nil
	}

	// Create the base directory and subdirectories
	rawDir := filepath.Join(baseDir, "raw")
	processedDir := filepath.Join(baseDir, "processed")

	if err := os.MkdirAll(rawDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create raw log directory: %w", err)
	}
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create processed log directory: %w", err)
	}

	logger.Info("Transcription file logging enabled",
		String("base_dir", baseDir),
		String("raw_dir", rawDir),
		String("processed_dir", processedDir))

	return fl, nil
}

// IsEnabled returns true if file logging is enabled
func (fl *FileLogger) IsEnabled() bool {
	return fl.baseDir != ""
}

// LogServiceStarted writes a service started header to both raw and processed logs
func (fl *FileLogger) LogServiceStarted(frequencyID string) error {
	if !fl.IsEnabled() {
		return nil
	}

	now := time.Now().Local()
	timeStr := now.Format("15:04:05")
	header := fmt.Sprintf("[%s] TRANSCRIPTION SERVICE STARTED\n================================\n", timeStr)

	// Write to both raw and processed logs
	if err := fl.writeHeader("raw", frequencyID, now, header); err != nil {
		return err
	}
	return fl.writeHeader("processed", frequencyID, now, header)
}

// LogRaw writes a raw transcription to the log file
func (fl *FileLogger) LogRaw(frequencyID string, timestamp time.Time, text string) error {
	if !fl.IsEnabled() {
		return nil
	}

	return fl.writeLog("raw", frequencyID, timestamp, text)
}

// LogProcessed writes a processed transcription to the log file
func (fl *FileLogger) LogProcessed(frequencyID string, timestamp time.Time, speakerType, callsign, text string) error {
	if !fl.IsEnabled() {
		return nil
	}

	// Format the processed entry with speaker info
	var entry string
	if callsign != "" {
		entry = fmt.Sprintf("[%s] %s: %s", speakerType, callsign, text)
	} else {
		entry = fmt.Sprintf("[%s] %s", speakerType, text)
	}

	return fl.writeLog("processed", frequencyID, timestamp, entry)
}

// writeHeader writes a header entry to the appropriate file (no timestamp prefix)
func (fl *FileLogger) writeHeader(subDir, frequencyID string, timestamp time.Time, header string) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Generate filename based on date and frequency ID
	localTimestamp := timestamp.Local()
	dateStr := localTimestamp.Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s.log", dateStr, frequencyID)
	filePath := filepath.Join(fl.baseDir, subDir, filename)

	// Check if we have this file cached
	cacheKey := filePath
	file, exists := fl.files[cacheKey]

	if !exists {
		// Open file for appending (create if doesn't exist)
		var err error
		file, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", filePath, err)
		}
		fl.files[cacheKey] = file
	}

	// Write header directly
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write to log file %s: %w", filePath, err)
	}

	// Sync to ensure data is written
	if err := file.Sync(); err != nil {
		fl.logger.Warn("Failed to sync log file",
			String("file", filePath),
			Error(err))
	}

	return nil
}

// writeLog writes a log entry to the appropriate file
func (fl *FileLogger) writeLog(subDir, frequencyID string, timestamp time.Time, text string) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Generate filename based on date and frequency ID
	localTimestamp := timestamp.Local()
	dateStr := localTimestamp.Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s.log", dateStr, frequencyID)
	filePath := filepath.Join(fl.baseDir, subDir, filename)

	// Check if we have this file cached
	cacheKey := filePath
	file, exists := fl.files[cacheKey]

	if !exists {
		// Open file for appending (create if doesn't exist)
		var err error
		file, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", filePath, err)
		}
		fl.files[cacheKey] = file
	}

	// Format the log entry with local machine timestamp
	timeStr := localTimestamp.Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s\n", timeStr, text)

	// Write to file
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log file %s: %w", filePath, err)
	}

	// Sync to ensure data is written
	if err := file.Sync(); err != nil {
		fl.logger.Warn("Failed to sync log file",
			String("file", filePath),
			Error(err))
	}

	return nil
}

// Close closes all open file handles
func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	var lastErr error
	for path, file := range fl.files {
		if err := file.Close(); err != nil {
			fl.logger.Error("Failed to close log file",
				String("file", path),
				Error(err))
			lastErr = err
		}
	}
	fl.files = make(map[string]*os.File)

	return lastErr
}

// CleanupOldFiles cleans up file handles for dates that are no longer current
// This should be called periodically to prevent file handle accumulation
func (fl *FileLogger) CleanupOldFiles() {
	if !fl.IsEnabled() {
		return
	}

	fl.mu.Lock()
	defer fl.mu.Unlock()

	today := time.Now().Format("2006-01-02")

	for path, file := range fl.files {
		// Extract date from filename
		filename := filepath.Base(path)
		if len(filename) >= 10 {
			fileDate := filename[:10]
			if fileDate != today {
				if err := file.Close(); err != nil {
					fl.logger.Warn("Failed to close old log file",
						String("file", path),
						Error(err))
				}
				delete(fl.files, path)
				fl.logger.Debug("Closed old log file",
					String("file", path))
			}
		}
	}
}
