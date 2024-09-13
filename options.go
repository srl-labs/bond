package bond

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
)

var (
	// An error is returned if Agent tries to enable
	// WithConfigAcknowledge option without streaming configs.
	ErrAckCfgAndNotStreamCfg = errors.New("agent cannot acknowledge configs unless it enables config stream")
	// An error is returned if Agent tries to enable
	// WithAutoUpdateConfigState option while acknowledging configs.
	ErrAckCfgAndAutoCfgState = errors.New("agent cannot automatically update config state while acknowledging configs")
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
		return nil
	}
}

// WithStreamConfig enables streaming of application configs for each YANG path.
// For example: the application will stream in separate configs
// for the root container (e.g. /greeter) and any YANG
// list entries (e.g. /greeter/list-node[name=entry1]).
// Streamed config notification contents are defined by the Config type.
// Application can receive streamed config notifications from channel Config.
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

// WithAutoUpdateConfigState enables SR Linux to
// automatically update telemetry state for app configs.
// When configs are commited, the config data will
// be merged and synced with the app's current state.
// Note: app will still need to manually
// update state using UpdateState or DeleteState for
// non-configurable (config=false) YANG schema nodes.
// Any json data pushed using UpdateState
// will override existing state for that YANG path.
// If all configs for a path (e.g. /greeter) are deleted,
// the app's state may still not be empty and will contain
// the json data that was last pushed by UpdateState.
// An error will be returned if WaitConfigAck is
// enabled because NDK server requires app configuration
// to always succeed during commit phase.
// By default, this option is disabled
// and state for configs need to be updated manually
// with UpdateState or DeleteState.
func WithAutoUpdateConfigState() Option {
	return func(a *Agent) error {
		a.autoCfgState = true
		return nil
	}
}

// WithCaching enables SR Linux to cache
// streamed notifications in NDK server.
// - By default, notifications are not cached
// and are streamed directly to the application.
// This optimizes memory usage for NDK
// server and is useful for apps that require
// a scaled environment. Notifications, such as
// Route and NextHopGroups, contain routing info,
// which can consume significant memory if cached.
// - If caching is enabled, notifications would contain a NDK
// operation Op of either Create, Update, or Delete.
// - If caching is disabled, Create or Update notifications,
// will instead have an op of CreateOrUpdate.
// Delete notification Data (e.g. route ownerid) will be nil.
// However, Delete notification Key contents will always be present.
// - Note: Config, Network instance, and App id notifications will
// always be cached in NDK server, regardless of WithCaching set.
// All other notifications will not be cached by default.
func WithCaching() Option {
	return func(a *Agent) error {
		a.cacheNotifications = true
		return nil
	}
}

// validateOptions validates the Agent's final configuration.
// A slice of errors is returned.
func (a *Agent) validateOptions() []error {
	var errs []error
	if a.configAck && !a.streamConfig {
		errs = append(errs, ErrAckCfgAndNotStreamCfg)
	} else if a.configAck && a.autoCfgState {
		errs = append(errs, ErrAckCfgAndAutoCfgState)
	}
	return errs
}
