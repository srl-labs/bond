package bond

import (
	"context"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

// ReceiveAppIdNotifications starts an AppId notification stream
// and sends notifications to channel `AppId`.
// If the main execution intends to continue running after calling this method,
// it should be called as a goroutine.
// `AppId` chan carries values of type ndk.AppIdentNotification
func (a *Agent) ReceiveAppIdNotifications(ctx context.Context) {
	defer close(a.Notifications.AppId)
	AppIdStream := a.startAppIdNotificationStream(ctx)

	for AppIdStreamResp := range AppIdStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(AppIdStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("AppId notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received AppId notifications:\n%s", b)

		for _, n := range AppIdStreamResp.GetNotifications() {
			AppIdNotif := n.GetAppId()
			if AppIdNotif == nil {
				a.logger.Info().
					Msgf("Empty AppId notification:%+v", n)
				continue
			}
			a.Notifications.AppId <- AppIdNotif
		}
	}
}

// startAppIdNotificationStream starts a notification stream for AppId service notifications.
func (a *Agent) startAppIdNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("AppId Notification stream created")

	a.addAppIdSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"AppId", streamChan)

	return streamChan
}

// addAppIdSubscription adds a subscription for AppId service notifications
// to the allocated notification stream.
func (a *Agent) addAppIdSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for AppId service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_OPERATION_ADD_SUBSCRIPTION,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_AppId{ // AppId service
			AppId: &ndk.AppIdentSubscriptionRequest{},
		},
	}

	registerResp, err := a.stubs.sdkMgrService.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_SDK_MGR_STATUS_SUCCESS {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}
