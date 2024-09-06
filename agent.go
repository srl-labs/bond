package bond

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	ndkSocket           = "unix:///opt/srlinux/var/run/sr_sdk_service_manager:50053"
	defaultRetryTimeout = 5 * time.Second

	defaultUsername = "admin"
	defaultPassword = "NokiaSrl1!"

	agentMetadataKey = "agent_name"
)

type Agent struct {
	ctx         context.Context
	cancel      context.CancelFunc
	Name        string
	AppID       uint32
	appRootPath string
	// paths contains all paths, in XPath format,
	// that are used to update the app's state data.
	// Possible keys include app root path
	// or any YANG lists.
	// e.g. /greeter, /greeter/list-node[name=entry1]
	paths map[string]struct{}

	gRPCConn        *grpc.ClientConn
	logger          *zerolog.Logger
	retryTimeout    time.Duration
	gNMITarget      *target.Target
	keepAliveConfig *keepAliveConfig

	// agent will stream configs individually for each XPath
	// instead of retrieving full app config
	streamConfig bool

	// SRLinux will wait for explicit acknowledgement
	// from app after delivering configuration.
	waitConfigAck bool

	// NDK Service client stubs
	stubs *stubs

	// NDK streamed notification channels
	Notifications *Notifications
}

// stubs contains NDK service client stubs
// used to call service methods.
type stubs struct {
	sdkMgrService       ndk.SdkMgrServiceClient
	notificationService ndk.SdkNotificationServiceClient
	telemetryService    ndk.SdkMgrTelemetryServiceClient
	routeService        ndk.SdkMgrRouteServiceClient
	nextHopGroupService ndk.SdkMgrNextHopGroupServiceClient
	configService       ndk.SdkMgrConfigServiceClient
}

// keepAliveConfig contains settings for keepalive messages.
// app will log every interval seconds
// until ndk mgr has failed >= threshold times.
type keepAliveConfig struct {
	interval  time.Duration
	threshold int
}

// IsSet returns whether Agent is configured with keepalives.
func (k *keepAliveConfig) IsSet() bool {
	return k != nil && k.interval != 0 && k.threshold != 0
}

// NewAgent creates a new Agent instance.
func NewAgent(name string, opts ...Option) (*Agent, []error) {
	var errs []error

	a := &Agent{
		Name:         name,
		retryTimeout: defaultRetryTimeout,
		paths:        make(map[string]struct{}),
		Notifications: &Notifications{
			FullConfigReceived: make(chan struct{}),
			Config:             make(chan *ConfigNotification),
			Interface:          make(chan *ndk.InterfaceNotification),
			Route:              make(chan *ndk.IpRouteNotification),
			NextHopGroup:       make(chan *ndk.NextHopGroupNotification),
			NwInst:             make(chan *ndk.NetworkInstanceNotification),
			Lldp:               make(chan *ndk.LldpNeighborNotification),
			Bfd:                make(chan *ndk.BfdSessionNotification),
			AppId:              make(chan *ndk.AppIdentNotification),
		},
	}

	// process all options and return cumulative errors
	for _, opt := range opts {
		if err := opt(a); err != nil {
			errs = append(errs, err)
		}
	}
	// validate final Agent configuration
	errs = append(errs, a.validateOptions()...)
	if len(errs) > 0 {
		return nil, errs
	}

	a.ctx = metadata.AppendToOutgoingContext(a.ctx, agentMetadataKey, a.Name)
	return a, errs
}

func (a *Agent) Start() error {
	// connect to NDK socket
	err := a.connect()
	if err != nil {
		return err
	}

	a.logger.Info().Msg("Connected to NDK socket")

	// create NDK client stubs
	a.stubs = &stubs{
		sdkMgrService:       ndk.NewSdkMgrServiceClient(a.gRPCConn),
		notificationService: ndk.NewSdkNotificationServiceClient(a.gRPCConn),
		telemetryService:    ndk.NewSdkMgrTelemetryServiceClient(a.gRPCConn),
		routeService:        ndk.NewSdkMgrRouteServiceClient(a.gRPCConn),
		nextHopGroupService: ndk.NewSdkMgrNextHopGroupServiceClient(a.gRPCConn),
		configService:       ndk.NewSdkMgrConfigServiceClient(a.gRPCConn),
	}

	// register agent
	err = a.register()
	if err != nil {
		return err
	}

	a.exitHandler() // exit gracefully if app stops

	// enable keepalives
	if a.keepAliveConfig.IsSet() {
		go a.keepAlive(a.ctx, a.keepAliveConfig.interval, a.keepAliveConfig.threshold)
	}

	a.newGNMITarget()

	go a.receiveConfigNotifications(a.ctx)

	return nil
}

// exitHandle handles when the application stops and receives interrupt/SIGTERM signals.
func (a *Agent) exitHandler() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig // blocking until app is stopped
		a.stop()
	}()
}

// stop performs graceful shutdown of the application.
// Actions performed include unregistering the agent with ndk server,
// closing the grpc channel, and closing the program context.
// All program goroutines will react to the context cancellation and exit.
func (a *Agent) stop() {
	defer a.cancel() // cancel app context
	a.logger.Info().
		Msg("Application has stopped and will exit gracefully.")
	// unregister agent
	err := a.unregister()
	if err != nil {
		a.logger.Error().
			Err(err).
			Msg("Application has failed to unregister.")
		return
	}
	// close gRPC connection
	err = a.gRPCConn.Close()
	if err != nil {
		a.logger.Error().
			Err(err).
			Msg("Closing gRPC connection to NDK server failed")
	}
}

// connect attempts connecting to the NDK socket.
func (a *Agent) connect() error {
	conn, err := grpc.Dial(ndkSocket,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	a.gRPCConn = conn

	return err
}

// register registers the agent with NDK.
func (a *Agent) register() error {
	req := &ndk.AgentRegistrationRequest{
		WaitConfigAck: a.waitConfigAck,
	}
	resp, err := a.stubs.sdkMgrService.AgentRegister(a.ctx, req)
	if err != nil || resp.Status != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Fatal().
			Err(err).
			Str("status", resp.GetStatus().String()).
			Msg("Agent registration failed")

		return fmt.Errorf("agent registration failed")
	}

	a.logger.Info().
		Uint32("app-id", resp.GetAppId()).
		Str("name", a.Name).
		Msg("Application registered successfully!")

	return nil
}

// unregister unregisters the agent from NDK.
func (a *Agent) unregister() error {
	r, err := a.stubs.sdkMgrService.AgentUnRegister(a.ctx, &ndk.AgentRegistrationRequest{})
	if err != nil || r.Status != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Fatal().
			Err(err).
			Str("status", r.GetStatus().String()).
			Msg("Agent unregistration failed")

		return fmt.Errorf("agent unregistration failed")
	}

	a.logger.Info().
		Uint32("app-id", r.GetAppId()).
		Str("name", a.Name).
		Msg("Application unregistered successfully!")

	return nil
}

// keepAlive sends periodic keepalive messages until NDK mgr has failed threshold times.
// SR Linux will respond with a status message: kSdkMgrSuccess or kSdkMgrFailed.
func (a *Agent) keepAlive(ctx context.Context, interval time.Duration, threshold int) {
	errCounter := 0
	timer := time.NewTicker(interval)
	retry := time.NewTicker(a.retryTimeout)
	for {
		select {
		case <-ctx.Done():
			retry.Stop()
			timer.Stop()
			a.logger.Info().
				Str("name", a.Name).
				Msg("context has been cancelled, agent stopped sending keepalives.")
			return
		case <-timer.C: // send keepalives every interval
			resp, err := a.stubs.sdkMgrService.KeepAlive(a.ctx, &ndk.KeepAliveRequest{})
			if err != nil { // retry RPC if failure
				a.logger.Info().
					Err(err).
					Str("status", resp.GetStatus().String()).
					Msg("Agent failed to send keepalives.")
				a.logger.Printf("agent %s retrying in %s", a.Name, a.retryTimeout)
				time.Sleep(a.retryTimeout)
				<-retry.C
				continue
			}
			status := resp.GetStatus()
			a.logger.Info().
				Str("name", a.Name).
				Msgf("Agent sent keepalive at %s and received response status: %s", time.Now(), status.String())
			if status == ndk.SdkMgrStatus_kSdkMgrFailed { // sdk_mgr has failed
				errCounter += 1
				if errCounter >= a.keepAliveConfig.threshold {
					a.logger.Info().
						Str("name", a.Name).
						Msgf("Agent keepalives have been stopped because sdk mgr has failed %d times.", threshold)
					return
				}
			} else { //sdk_mgr status is success
				errCounter = 0
			}
		}
	}
}
