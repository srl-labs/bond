package bond

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	commitEndKeyPath = ".commit.end"
)

// receiveConfigNotifications receives a stream of configuration notifications
// buffer them in the configuration buffer and populates ConfigState struct of the App
// once the whole committed config is received.

func (a *Agent) receiveConfigNotifications(ctx context.Context) {
	configStream := a.StartConfigNotificationStream(ctx)

	for cfgStreamResp := range configStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(cfgStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Config notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received notifications:\n%s", b)

		a.handleConfigNotifications(cfgStreamResp)
	}
}

// StartConfigNotificationStream starts a notification stream for Config service notifications.
func (a *Agent) StartConfigNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Notification stream created")

	a.addConfigSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"config", streamChan)

	return streamChan
}

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

// addConfigSubscription adds a subscription for Config service notifications
// to the allocated notification stream.

func (a *Agent) addConfigSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for Config service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_AddSubscription,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_Config{ // config service
			Config: &ndk.ConfigSubscriptionRequest{},
		},
	}

	registerResp, err := a.SDKMgrServiceClient.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}

// handleConfigNotifications buffers the configuration notifications received
// from the config notification stream before commit end notification is received.
func (a *Agent) handleConfigNotifications(
	notifStreamResp *ndk.NotificationStreamResponse,
) {
	notifs := notifStreamResp.GetNotification()

	for _, n := range notifs {
		cfgNotif := n.GetConfig()
		if cfgNotif == nil {
			a.logger.Info().
				Msgf("Empty configuration notification:%+v", n)
			continue
		}

		// if cfgNotif.Key.JsPath != commitEndKeyPath {
		// 	a.logger.Debug().
		// 		Msgf("Handling config notification: %+v", cfgNotif)

		// 	a.handleConfigtopusConfig(cfgNotif)
		// }

		// commit.end notification is received and it is not a zero commit sequence
		// this means that the full config is received and we can process it
		if cfgNotif.Key.JsPath == commitEndKeyPath &&
			!a.isCommitSeqZero(cfgNotif.GetData().GetJson()) {
			a.logger.Debug().
				Msgf("Received commit end notification: %+v", cfgNotif)

			a.ConfigReceivedCh <- struct{}{}
		}
	}
}

type CommitSeq struct {
	CommitSeq int `json:"commit_seq"`
}

// isCommitSeqZero checks if the commit sequence passed in the jsonStr is zero.
func (a *Agent) isCommitSeqZero(jsonStr string) bool {
	var commitSeq CommitSeq

	err := json.Unmarshal([]byte(jsonStr), &commitSeq)
	if err != nil {
		a.logger.Error().Msgf("failed to unmarshal json: %s", err)
		return false
	}

	return commitSeq.CommitSeq == 0
}

// isEmptyObject checks if the jsonStr is an empty object.
func (a *Agent) isEmptyObject(jsonStr string) bool {
	var obj map[string]any

	err := json.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		a.logger.Error().Msgf("failed to unmarshal json: %s", err)
		return false
	}

	if len(obj) == 0 {
		return true
	}

	return false
}
