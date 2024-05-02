package bond

import (
	"context"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

// ReceiveIntfNotifications starts an interface notification stream
// and sends notifications to channel `Interface`.
// If the main execution intends to continue running after calling this method,
// it should be called as a goroutine.
// `Interface` chan carries values of type ndk.InterfaceNotification.
func (a *Agent) ReceiveIntfNotifications(ctx context.Context) {
	defer close(a.Notifs.Interface)
	intfStream := a.startInterfaceNotificationStream(ctx)

	for intfStreamResp := range intfStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(intfStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Interface notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received notifications:\n%s", b)

		for _, n := range intfStreamResp.GetNotification() {
			intfNotif := n.GetIntf()
			if intfNotif == nil {
				a.logger.Info().
					Msgf("Empty interface notification:%+v", n)
				continue
			}
			a.Notifs.Interface <- intfNotif
		}
	}
}

// startInterfaceNotificationStream starts a notification stream for Intf service notifications.
func (a *Agent) startInterfaceNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Notification stream created")

	a.addIntfSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"interface", streamChan)

	return streamChan
}

// addIntfSubscription adds a subscription for Interface service notifications
// to the allocated notification stream.
func (a *Agent) addIntfSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for Intf service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_AddSubscription,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_Intf{ // intf service
			Intf: &ndk.InterfaceSubscriptionRequest{},
		},
	}

	registerResp, err := a.SDKMgrServiceClient.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}
