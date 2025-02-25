package log

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"

	charmlog "github.com/charmbracelet/log"
)

const (
	FormatJSON   string = "json"
	FormatLogfmt string = "logfmt"
	FormatText   string = "text"
)

var (
	ErrUnknownLogLevel  = errors.New("unknown log level")
	ErrUnknownLogFormat = errors.New("unknown log format")
)

// CreateHandler creates a [slog.Handler] by strings.
func CreateHandler(w io.Writer, logLevel, logFormat string) (slog.Handler, error) {
	level, err := GetLevel(logLevel)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(logFormat) {
	case FormatJSON:
		return slog.NewJSONHandler(w, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		}), nil
	case FormatLogfmt:
		return slog.NewTextHandler(w, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		}), nil
	case FormatText:
		return newCharmLogHandler(w, level), nil
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

func newCharmLogHandler(w io.Writer, level slog.Level) slog.Handler {
	//nolint:gosec // G115: input from GetLevel.
	lvl := int32(level)

	logger := charmlog.NewWithOptions(w, charmlog.Options{
		Level:        charmlog.Level(lvl),
		Formatter:    charmlog.TextFormatter,
		ReportCaller: true,
	})
	if isatty.IsTerminal(os.Stdout.Fd()) {
		logger.SetColorProfile(termenv.ANSI256)
	}

	return logger
}
