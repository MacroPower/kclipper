package log

import (
	"log/slog"
	"os"
	"strings"
)

const (
	JSONFormat = "json"
	TextFormat = "text"
)

// CreateHandler creates a [slog.Handler] by strings.
func CreateHandler(logLevel, logFormat string) slog.Handler {
	level := GetLevel(logLevel)

	switch strings.ToLower(logFormat) {
	case JSONFormat:
		return slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	case TextFormat:
		return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	default:
		return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
}

func GetLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "panic":
		return slog.LevelError
	case "fatal":
		return slog.LevelError
	case "error":
		return slog.LevelError
	case "warn", "warning":
		return slog.LevelWarn
	case "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	case "trace":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}
