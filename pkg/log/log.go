package log

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	JSONFormat = "json"
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

// SetLogFormat sets a log/slog format.
func SetLogFormat(logFormat string) {
	switch strings.ToLower(logFormat) {
	case JSONFormat:
		os.Setenv("ARGOCD_LOG_FORMAT", JSONFormat)
	case TextFormat, "":
		os.Setenv("ARGOCD_LOG_FORMAT", TextFormat)
	default:
		panic(fmt.Errorf("unknown log format '%s'", logFormat))
	}

	slog.SetDefault(NewWithCurrentConfig())
}

// SetLogLevel parses and sets a log/slog level.
func SetLogLevel(logLevel string) {
	level := GetLevel(logLevel)
	os.Setenv("ARGOCD_LOG_LEVEL", level.String())
	slog.SetLogLoggerLevel(level)
}
