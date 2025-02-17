package log

import (
	"errors"
	"log/slog"
	"os"
	"strings"
)

const (
	JSONFormat = "json"
	TextFormat = "text"
)

var (
	ErrUnknownLogLevel  = errors.New("unknown log level")
	ErrUnknownLogFormat = errors.New("unknown log format")
)

// CreateHandler creates a [slog.Handler] by strings.
func CreateHandler(logLevel, logFormat string) (slog.Handler, error) {
	level, err := GetLevel(logLevel)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	}

	switch strings.ToLower(logFormat) {
	case JSONFormat:
		return slog.NewJSONHandler(os.Stderr, opts), nil
	case TextFormat:
		return slog.NewTextHandler(os.Stderr, opts), nil
	}

	return nil, ErrUnknownLogFormat
}

func GetLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "error":
		return slog.LevelError, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	}

	return 0, ErrUnknownLogLevel
}
