package bond

import (
	"context"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

// ReceiveNwInstNotifications starts an network instance notification
// stream and sends notifications to channel `NwInst`.
// If the main execution intends to continue running after calling this method,
// it should be called as a goroutine.
// `NwInst` chan carries values of type ndk.NetworkInstanceNotification
func (a *Agent) ReceiveNwInstNotifications(ctx context.Context) {
	defer close(a.Notifs.NwInst)
	nwInstStream := a.startNwInstNotificationStream(ctx)

	for nwInstStreamResp := range nwInstStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(nwInstStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Network instance notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received network instance notifications:\n%s", b)

		for _, n := range nwInstStreamResp.GetNotification() {
			nwInstNotif := n.GetNwInst()
			if nwInstNotif == nil {
				a.logger.Info().
					Msgf("Empty network instance notification:%+v", n)
				continue
			}
			a.Notifs.NwInst <- nwInstNotif
		}
	}
}

// startNwInstNotificationStream starts a notification stream for Network Instance service notifications.
func (a *Agent) startNwInstNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Network Instance notification stream created")

	a.addNwInstSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"nwinst", streamChan)

	return streamChan
}

// addNwInstSubscription adds a subscription for Network Instance service notifications
// to the allocated notification stream.
func (a *Agent) addNwInstSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for nwinst service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_AddSubscription,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_NwInst{ // nwinst service
			NwInst: &ndk.NetworkInstanceSubscriptionRequest{},
		},
	}

	registerResp, err := a.SDKMgrServiceClient.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}
