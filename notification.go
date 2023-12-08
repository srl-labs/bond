package bond

import (
	"context"
	"io"
	"time"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

// createNotificationStream creates a notification stream and returns the Stream ID.
// Stream ID is used to register notifications for other services.
// It retries with retryTimeout until it succeeds.
func (a *Agent) createNotificationStream(ctx context.Context) uint64 {
	retry := time.NewTicker(a.retryTimeout)

	for {
		// get subscription and streamID
		notificationResponse, err := a.SDKMgrServiceClient.NotificationRegister(ctx,
			&ndk.NotificationRegisterRequest{
				Op: ndk.NotificationRegisterRequest_Create,
			})
		if err != nil || notificationResponse.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
			a.logger.Printf("agent %q could not register for notifications: %v. Status: %s",
				a.Name, err, notificationResponse.GetStatus().String())
			a.logger.Printf("agent %q retrying in %s", a.Name, a.retryTimeout)

			<-retry.C // retry timer
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

	retry := time.NewTicker(a.retryTimeout)
	streamClient := a.getNotificationStreamClient(ctx, streamID)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			streamResp, err := streamClient.Recv()
			if err == io.EOF {
				a.logger.Printf("agent %s received EOF for stream %v", a.Name, subscType)
				a.logger.Printf("agent %s retrying in %s", a.Name, a.retryTimeout)

				<-retry.C // retry timer
				continue
			}
			if err != nil {
				a.logger.Printf("agent %s failed to receive notification: %v", a.Name, err)

				<-retry.C // retry timer
				continue
			}
			streamChan <- streamResp
		}
	}
}

// getNotificationStreamClient acquires the notification stream client that is used to receive
// streamed notifications.
func (a *Agent) getNotificationStreamClient(ctx context.Context, streamID uint64) ndk.SdkNotificationService_NotificationStreamClient {
	retry := time.NewTicker(a.retryTimeout)

	for {
		streamClient, err := a.NotificationServiceClient.NotificationStream(ctx,
			&ndk.NotificationStreamRequest{
				StreamId: streamID,
			})
		if err != nil {
			a.logger.Info().Msgf("agent %s failed creating stream client with stream ID=%d: %v", a.Name, streamID, err)
			a.logger.Printf("agent %s retrying in %s", a.Name, a.retryTimeout)
			time.Sleep(a.retryTimeout)

			<-retry.C // retry timer
			continue
		}

		return streamClient
	}
}
