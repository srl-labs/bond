package bond

import (
	"context"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

// ReceiveNexthopGroupNotifications starts a next hop group notification stream
// and sends notifications to channel `NextHopGroup`.
// If the main execution intends to continue running after calling this method,
// it should be called as a goroutine.
// `NextHopGroup` chan carries values of type ndk.NextHopGroupNotification
func (a *Agent) ReceiveNexthopGroupNotifications(ctx context.Context) {
	defer close(a.Notifications.NextHopGroup)
	nhgStream := a.startNhgNotificationStream(ctx)

	for nhgStreamResp := range nhgStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(nhgStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Nexthop group notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received Nexthop group notifications:\n%s", b)

		for _, n := range nhgStreamResp.GetNotification() {
			nhgNotif := n.GetNhg()
			if nhgNotif == nil {
				a.logger.Info().
					Msgf("Empty Nexthop group notification:%+v", n)
				continue
			}
			a.Notifications.NextHopGroup <- nhgNotif
		}
	}
}

// startNhgNotificationStream starts a notification stream for Nexthop Group service notifications.
func (a *Agent) startNhgNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Nhg Notification stream created")

	a.addNhgSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"nhg", streamChan)

	return streamChan
}

// addNhgSubscription adds a subscription for Nexthop Group service notifications
// to the allocated notification stream.
func (a *Agent) addNhgSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for nhg service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_AddSubscription,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_Nhg{ // nhg service
			Nhg: &ndk.NextHopGroupSubscriptionRequest{},
		},
	}

	registerResp, err := a.SDKMgrServiceClient.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}
