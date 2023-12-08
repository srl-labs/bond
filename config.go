package bond

import (
	"context"
	"encoding/json"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

const (
	commitEndKeyPath = ".commit.end"
)

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
