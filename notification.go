package bond

import (
	"context"
	"io"
	"time"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

// Notifications contains channels for various NDK notifications.
// By default, the entire app's configs is stored in config buffer.
// To populate channels for other notification types (e.g. interface),
// explicit calls to `Receive<type>Notifications` methods are required.
type Notifications struct {
	// FullConfigReceived chan receives the value and stores in FullConfig
	// when the entire application's config is received by the stream client.
	//
	// This channel will not be used if streaming of configs
	// is enabled with WithStreamConfig option.
	FullConfigReceived chan struct{}

	// FullConfig holds the application's config as json_ietf encoded string
	// that is retrieved from the gNMI server once the commit is done.
	// Applications are expected to read from this buffer to populate
	// their Config and State struct.
	//
	// This buffer will not be used if streaming of configs
	// is enabled with WithStreamConfig option.
	FullConfig []byte

	// Config chan receives streamed config notifications for each individual app path.
	// The contents of each notification is defined by ConfigNotification type.
	// To stream configs, application has to register
	// Agent with option WithStreamConfig.
	// Otherwise, individual configs will not be streamed and the entire
	// config will be populated to the FullConfig buffer.
	// bond does not allow application to simultaneously
	// stream individual configs while also receiving full config.
	//
	// This channel will not be used if Agent does not
	// have WithStreamConfig option set.
	Config chan *ConfigNotification

	// Interface chan receives streamed interface notifications.
	// Method ReceiveInterfaceNotifications starts stream
	// and populates notifications in chan Interface.
	Interface chan *ndk.InterfaceNotification

	// Route chan receives streamed route notifications.
	// Method ReceiveRouteNotifications starts stream
	// and populates notifications in chan Route.
	Route chan *ndk.IpRouteNotification

	// NextHopGroup chan receives streamed next hop group notifications.
	// Method ReceiveNexthopGroupNotifications starts stream
	// and populates notifications in chan NextHopGroup.
	NextHopGroup chan *ndk.NextHopGroupNotification

	// NwInst chan receives streamed network instance notifications.
	// Method ReceiveNetworkInstanceNotifications starts stream
	// and populates notifications in chan NwInst.
	NwInst chan *ndk.NetworkInstanceNotification

	// Lldp chan receives streamed LLDP neighbor notifications.
	// Method ReceiveLLDPNotifications starts stream
	// and populates notifications in chan Lldp.
	Lldp chan *ndk.LldpNeighborNotification

	// Bfd chan receives streamed Bfd Session notifications.
	// Method ReceiveBfdNotifications starts stream
	// and populates notifications in chan Bfd.
	Bfd chan *ndk.BfdSessionNotification

	// AppId chan receives streamed App identifier notifications.
	// Method ReceiveAppIdNotifications starts stream
	// and populates notifications in chan AppId.
	AppId chan *ndk.AppIdentNotification
}

// createNotificationStream creates a notification stream and returns the Stream ID.
// Stream ID is used to register notifications for other services.
// It retries with retryTimeout until it succeeds.
func (a *Agent) createNotificationStream(ctx context.Context) uint64 {
	for {
		// get subscription and streamID
		notificationResponse, err := a.stubs.sdkMgrService.NotificationRegister(ctx,
			&ndk.NotificationRegisterRequest{
				Op: ndk.NotificationRegisterRequest_OPERATION_CREATE,
			})
		if err != nil || notificationResponse.GetStatus() != ndk.SdkMgrStatus_SDK_MGR_STATUS_SUCCESS {
			a.logger.Printf("agent %q could not register for notifications: %v. Status: %s",
				a.Name, err, notificationResponse.GetStatus().String())
			a.logger.Printf("agent %q retrying in %s", a.Name, a.retryTimeout)

			time.Sleep(a.retryTimeout)

			continue
		}

		return notificationResponse.GetStreamId()
	}
}

// startNotificationStream starts a notification stream for a given NotificationRegisterRequest
// and sends the received notifications to the passed channel.
func (a *Agent) startNotificationStream(ctx context.Context,
	streamID uint64,
	subscType string,
	streamChan chan *ndk.NotificationStreamResponse,
) {
	defer close(streamChan)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Str("subscription-type", subscType).
		Msg("Starting streaming notifications")

	streamClient := a.getNotificationStreamClient(ctx, streamID)

	for {
		streamResp, err := streamClient.Recv()

		select {
		case <-ctx.Done():
			a.logger.Info().
				Uint64("stream-id", streamID).
				Str("subscription-type", subscType).
				Msg("agent context has cancelled, exiting notification stream")
			return
		default:
			if err == io.EOF {
				a.logger.Info().
					Uint64("stream-id", streamID).
					Str("subscription-type", subscType).
					Msgf("received EOF, retrying in %s", a.retryTimeout)

				time.Sleep(a.retryTimeout)

				continue
			}

			if err != nil {
				a.logger.Error().
					Err(err).
					Str("timestamp", time.Now().String()).
					Uint64("stream-id", streamID).
					Str("subscription-type", subscType).
					Msgf("failed to receive notification, retrying in %s", a.retryTimeout)

				time.Sleep(a.retryTimeout)

				continue
			}

			streamChan <- streamResp
		}
	}
}

// getNotificationStreamClient acquires the notification stream client that is used to receive
// streamed notifications.
func (a *Agent) getNotificationStreamClient(ctx context.Context, streamID uint64) ndk.SdkNotificationService_NotificationStreamClient {
	for {
		streamClient, err := a.stubs.notificationService.NotificationStream(ctx,
			&ndk.NotificationStreamRequest{
				StreamId: streamID,
			})
		if err != nil {
			a.logger.Info().Msgf("agent %s failed creating stream client with stream ID=%d: %v", a.Name, streamID, err)
			a.logger.Printf("agent %s retrying in %s", a.Name, a.retryTimeout)

			time.Sleep(a.retryTimeout)

			continue
		}

		return streamClient
	}
}
