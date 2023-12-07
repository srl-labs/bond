package bond

import (
	"context"

	"github.com/rs/zerolog"
)

type Option func(*Agent) error

// WithLogger sets the logger for the Agent
func WithLogger(logger *zerolog.Logger) Option {
	return func(a *Agent) error {
		a.logger = logger
		return nil
	}
}

func WithContext(ctx context.Context) Option {
	return func(a *Agent) error {
		a.ctx = ctx
		return nil
	}
}
