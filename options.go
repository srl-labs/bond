package bond

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

type Option func(*Agent) error

type optionSet func(*Agent) bool

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
		a.keepAliveConfig = &keepAliveConfig{
			interval:  interval,
			threshold: threshold,
		}
		return nil
	}
}

// isKeepAliveSet returns whether Agent is configured with keepalives.
func isKeepAliveSet() optionSet {
	return func(a *Agent) bool {
		return a.keepAliveConfig != nil &&
			a.keepAliveConfig.interval != 0 &&
			a.keepAliveConfig.threshold != 0
	}
}
