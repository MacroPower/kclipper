package cli

import (
	"os"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/text"
	log "github.com/sirupsen/logrus"

	utillog "github.com/MacroPower/kclx/pkg/argoutil/log"
)

// SetLogFormat sets a logrus log format
func SetLogFormat(logFormat string) {
	switch strings.ToLower(logFormat) {
	case utillog.JsonFormat:
		os.Setenv("ARGOCD_LOG_FORMAT", utillog.JsonFormat)
	case utillog.TextFormat, "":
		os.Setenv("ARGOCD_LOG_FORMAT", utillog.TextFormat)
	default:
		log.Fatalf("Unknown log format '%s'", logFormat)
	}

	log.SetFormatter(utillog.CreateFormatter(logFormat))
}

// SetLogLevel parses and sets a logrus log level
func SetLogLevel(logLevel string) {
	level, err := log.ParseLevel(text.FirstNonEmpty(logLevel, log.InfoLevel.String()))
	// errors.CheckError(err)
	if err != nil {
		panic(err)
	}
	os.Setenv("ARGOCD_LOG_LEVEL", level.String())
	log.SetLevel(level)
}
