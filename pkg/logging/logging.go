package logging

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

type Log struct {
	*zerolog.Logger
}

// setupOnce guards the package-level zerolog configuration so concurrent
// calls to New() don't race on the global settings.
var setupOnce sync.Once

func New() *Log {
	setupOnce.Do(func() {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	})

	log := zerolog.New(os.Stderr).With().Timestamp().Logger()
	return &Log{
		&log,
	}
}

func (l *Log) ErrorStack(err error) {
	l.Error().Caller(1).Stack().Err(err).Send()
}
