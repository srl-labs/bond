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

// WithContext sets the context and it's cancellation function for the Agent.
// The context will be cancelled automatically when the application
// is stopped and receives interrupt or SIGTERM signals.
func WithContext(ctx context.Context, cancel context.CancelFunc) Option {
	return func(a *Agent) error {
		if ctx == nil || cancel == nil {
			return errors.New("setting agent context failed. context cannot be nil")
		}
		a.ctx = ctx
		a.cancel = cancel
		return nil
	}
}

// WithAppRootPath sets the root XPATH path for the application configuration.
func WithAppRootPath(path string) Option {
	return func(a *Agent) error {
		a.appRootPath = path
		// also add the app's root path to combined list of app's paths
		a.paths[path] = struct{}{}
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
