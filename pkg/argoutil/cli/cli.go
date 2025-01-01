package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	utillog "github.com/MacroPower/kclx/pkg/argoutil/log"
)

// SetLogFormat sets a log/slog format
func SetLogFormat(logFormat string) {
	switch strings.ToLower(logFormat) {
	case utillog.JsonFormat:
		os.Setenv("ARGOCD_LOG_FORMAT", utillog.JsonFormat)
	case utillog.TextFormat, "":
		os.Setenv("ARGOCD_LOG_FORMAT", utillog.TextFormat)
	default:
		panic(fmt.Errorf("unknown log format '%s'", logFormat))
	}

	slog.SetDefault(utillog.NewWithCurrentConfig())
}

// SetLogLevel parses and sets a log/slog level
func SetLogLevel(logLevel string) {
	level := utillog.GetLevel(logLevel)
	os.Setenv("ARGOCD_LOG_LEVEL", level.String())
	slog.SetLogLoggerLevel(level)
}
