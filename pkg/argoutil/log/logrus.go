package log

import (
	"log/slog"
	"os"
	"strings"
)

const (
	JsonFormat = "json"
	TextFormat = "text"
)

// NewWithCurrentConfig creates a [slog.Logger] by using current configuration.
func NewWithCurrentConfig() *slog.Logger {
	h := CreateHandler(os.Getenv("ARGOCD_LOG_LEVEL"), os.Getenv("ARGOCD_LOG_FORMAT"))
	return slog.New(h)
}

// CreateHandler creates a [slog.Handler] by strings.
func CreateHandler(logLevel, logFormat string) slog.Handler {
	level := GetLevel(logLevel)

	switch strings.ToLower(logFormat) {
	case JsonFormat:
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
