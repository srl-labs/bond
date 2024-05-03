package bond

import (
	"context"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

// ReceiveBfdNotifications starts an Bfd Session notification
// stream and sends notifications to channel `Bfd`.
// If the main execution intends to continue running after calling this method,
// it should be called as a goroutine.
// `Bfd` chan carries values of type ndk.BfdSessionNotification
func (a *Agent) ReceiveBfdNotifications(ctx context.Context) {
	defer close(a.Notifications.Bfd)
	BfdStream := a.startBfdNotificationStream(ctx)

	for BfdStreamResp := range BfdStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(BfdStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Bfd Session notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received Bfd Session notifications:\n%s", b)

		for _, n := range BfdStreamResp.GetNotification() {
			BfdNotif := n.GetBfdSession()
			if BfdNotif == nil {
				a.logger.Info().
					Msgf("Empty Bfd Session notification:%+v", n)
				continue
			}
			a.Notifications.Bfd <- BfdNotif
		}
	}
}

// startBfdNotificationStream starts a notification stream
// for Bfd Session service notifications.
func (a *Agent) startBfdNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Bfd Session notification stream created")

	a.addBfdSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"bfdSession", streamChan)

	return streamChan
}

// addBfdSubscription adds a subscription for Bfd Session service
// notifications to the allocated notification stream.
func (a *Agent) addBfdSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for Bfd service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_AddSubscription,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_BfdSession{ // Bfd service
			BfdSession: &ndk.BfdSessionSubscriptionRequest{},
		},
	}

	registerResp, err := a.stubs.sdkMgrService.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}
