package bond

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
)

type Option func(*Agent) error

// WithLogger sets the logger for the Agent.
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

// WithAppRootPath sets the root XPATH path for the application configuration.
func WithAppRootPath(path string) Option {
	return func(a *Agent) error {
		a.appRootPath = path
		return nil
	}
}

// WithKeepAlive enables keepalive messages for the application configuration.
// Every interval seconds, app will send keepalive messages
// until ndk mgr has failed threshold times.
func WithKeepAlive(interval time.Duration, threshold int) Option {
	return func(a *Agent) error {
		if interval == 0 && threshold == 0 {
			return errors.New("configuring agent keepalives failed. interval and threshold cannot both be zero")
		}
		a.keepAliveConfig = &keepAliveConfig{
			interval:  interval,
			threshold: threshold,
		}
		return nil
	}
}
