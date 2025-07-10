package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Field aliases for zap fields
type Field = zapcore.Field

// Helper functions for creating fields
var (
	// String creates a field with a string value
	String = zap.String
	// Int creates a field with an int value
	Int = zap.Int
	// Int64 creates a field with an int64 value
	Int64 = zap.Int64
	// Float64 creates a field with a float64 value
	Float64 = zap.Float64
	// Bool creates a field with a bool value
	Bool = zap.Bool
	// Time creates a field with a time.Time value
	Time = zap.Time
	// Duration creates a field with a time.Duration value
	Duration = zap.Duration
	// Error creates a field with an error value
	Error = zap.Error
	// Any creates a field with any value
	Any = zap.Any
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
}

// Config represents logger configuration
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, console
}

// Custom level encoder that adds colors for console output
func coloredLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch level {
	case zapcore.ErrorLevel:
		enc.AppendString("\033[1;31m" + level.String() + "\033[0m") // Bold Red
	case zapcore.WarnLevel:
		enc.AppendString("\033[1;33m" + level.String() + "\033[0m") // Bold Yellow
	case zapcore.InfoLevel:
		enc.AppendString("\033[1;36m" + level.String() + "\033[0m") // Bold Cyan
	case zapcore.DebugLevel:
		enc.AppendString("\033[1;37m" + level.String() + "\033[0m") // Bold White
	default:
		enc.AppendString(level.String())
	}
}

// Custom name encoder that truncates or pads the logger name to a fixed width
func fixedWidthNameEncoder(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
	// Extract the last component of the logger name for shorter display
	parts := strings.Split(loggerName, ".")
	displayName := parts[len(parts)-1]

	// Limit to 15 chars and pad with spaces to align columns
	if len(displayName) > 15 {
		displayName = displayName[:15]
	} else if len(displayName) < 15 {
		displayName = displayName + strings.Repeat(" ", 15-len(displayName))
	}

	enc.AppendString(displayName)
}

// New creates a new logger with the given configuration
func New(config Config) (*Logger, error) {
	// Parse log level
	level, err := parseLogLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	// Set encoding options based on format
	if config.Format == "console" {
		encoderConfig.EncodeLevel = coloredLevelEncoder
		encoderConfig.EncodeName = fixedWidthNameEncoder
	} else {
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoderConfig.EncodeName = zapcore.FullNameEncoder
	}

	// Only include caller info for debug level
	// For other levels, omit the caller info
	if level != zapcore.DebugLevel {
		encoderConfig.CallerKey = zapcore.OmitKey
	} else {
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	// Create encoder based on format
	var encoder zapcore.Encoder
	switch config.Format {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	case "console":
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		return nil, fmt.Errorf("unsupported log format: %s", config.Format)
	}

	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	// Create logger options
	opts := []zap.Option{
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	// Only add caller info for debug level
	if level == zapcore.DebugLevel {
		opts = append(opts, zap.AddCaller(), zap.AddCallerSkip(1))
	}

	// Create logger
	logger := zap.New(core, opts...)

	return &Logger{Logger: logger}, nil
}

// parseLogLevel parses the log level string
func parseLogLevel(level string) (zapcore.Level, error) {
	switch level {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unsupported log level: %s", level)
	}
}

// With returns a logger with the given fields
func (l *Logger) With(fields ...zapcore.Field) *Logger {
	return &Logger{Logger: l.Logger.With(fields...)}
}

// Named returns a logger with the given name
func (l *Logger) Named(name string) *Logger {
	return &Logger{Logger: l.Logger.Named(name)}
}

// WithRequestID returns a logger with the request ID field
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.With(zap.String("request_id", requestID))
}

// WithError returns a logger with the error field
func (l *Logger) WithError(err error) *Logger {
	return l.With(zap.Error(err))
}
