package bond

import (
	"context"
	"fmt"
	"time"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"github.com/openconfig/gnmic/pkg/target"
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
	ctx context.Context

	Name        string
	AppID       uint32
	appRootPath string

	gRPCConn     *grpc.ClientConn
	logger       *zerolog.Logger
	retryTimeout time.Duration

	gNMITarget *target.Target

	// NDK Service clients
	SDKMgrServiceClient       ndk.SdkMgrServiceClient
	NotificationServiceClient ndk.SdkNotificationServiceClient
	TelemetryServiceClient    ndk.SdkMgrTelemetryServiceClient

	// configReceivedCh chan receives the value when the full config
	// is received by the stream client.
	ConfigReceivedCh chan struct{}
	// Config holds the application's config as json_ietf encoded string
	// that is retrieved from the gNMI server once the commit is done.
	// Applications are expected to read from this buffer to populate
	// their Config and State struct.
	Config []byte
}

// NewAgent creates a new Agent instance.
func NewAgent(name string, opts ...Option) (*Agent, error) {
	a := &Agent{
		Name:             name,
		retryTimeout:     defaultRetryTimeout,
		ConfigReceivedCh: make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(a); err != nil {
			return nil, err
		}
	}

	a.ctx = metadata.AppendToOutgoingContext(a.ctx, agentMetadataKey, a.Name)

	return a, nil
}

func (a *Agent) Start() error {
	// connect to NDK socket
	err := a.connect()
	if err != nil {
		return err
	}

	a.logger.Info().Msg("Connected to NDK socket")

	a.SDKMgrServiceClient = ndk.NewSdkMgrServiceClient(a.gRPCConn)
	a.NotificationServiceClient = ndk.NewSdkNotificationServiceClient(a.gRPCConn)
	a.TelemetryServiceClient = ndk.NewSdkMgrTelemetryServiceClient(a.gRPCConn)

	// register agent
	err = a.register()
	if err != nil {
		return err
	}

	a.newGNMITarget()

	go a.receiveConfigNotifications(a.ctx)

	return nil
}

func (a *Agent) Stop() error {
	// register agent
	err := a.unregister()
	if err != nil {
		return err
	}

	// close gRPC connection
	err = a.gRPCConn.Close()

	return err
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
	r, err := a.SDKMgrServiceClient.AgentRegister(a.ctx, &ndk.AgentRegistrationRequest{})
	if err != nil || r.Status != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Fatal().
			Err(err).
			Str("status", r.GetStatus().String()).
			Msg("Agent registration failed")

		return fmt.Errorf("agent registration failed")
	}

	a.logger.Info().
		Uint32("app-id", r.GetAppId()).
		Str("name", a.Name).
		Msg("Application registered successfully!")

	return nil
}

// unregister unregisters the agent from NDK.
func (a *Agent) unregister() error {
	r, err := a.SDKMgrServiceClient.AgentUnRegister(a.ctx, &ndk.AgentRegistrationRequest{})
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
