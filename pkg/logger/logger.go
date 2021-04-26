// Package logger sets logger.
package logger

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Setup is configuring the logger.
func Setup() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	log.Logger = zerolog.New(os.Stderr).With().Caller().Logger()

	rawLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))

	logLevel, err := zerolog.ParseLevel(rawLevel)
	if err != nil || rawLevel == "" {
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)

	log.Trace().Msgf("Log level set to %s.", logLevel)
}
