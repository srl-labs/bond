package bond

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
)

// An error is returned if Agent tries to enable
// WithConfigAcknowledge option without streaming configs.
var ErrAckCfgAndNotStreamCfg = errors.New("agent cannot acknowledge configs unless it enables config stream")

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

// WithStreamConfig enables streaming of application configs for each YANG path.
// For example: the application will stream in separate configs
// for the root container (e.g. /greeter) and any YANG
// list entries (e.g. /greeter/list-node[name=entry1]).
// Streamed config notification contents are defined by the Config type.
// Application can receive streamed config notifications from channel Config.
//
// The Agent does not stream config notifications by default.
// Instead, the agent will receive the app's entire configuration
// and populate the FullConfig buffer.
func WithStreamConfig() Option {
	return func(a *Agent) error {
		a.streamConfig = true
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

// WithConfigAcknowledge enables SR Linux to wait for explicit
// acknowledgement from app after delivering configuration.
// After config notifications are streamed in, app will need
// acknowledge config with `AcknowledgeConfig` method.
// By default, SR Linux will not wait for acknowledgement from app
// and will commit complete immediately.
func WithConfigAcknowledge() Option {
	return func(a *Agent) error {
		a.configAck = true
		return nil
	}
}

// validateOptions validates the Agent's final configuration.
// A slice of errors is returned.
func (a *Agent) validateOptions() []error {
	var errs []error
	if a.configAck && !a.streamConfig {
		errs = append(errs, ErrAckCfgAndNotStreamCfg)
	}
	return errs
}
