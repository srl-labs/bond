package bond

import (
	"context"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

// ReceiveLLDPNotifications starts an LLDP neighbor notification
// stream and sends notifications to channel `Lldp`.
// If the main execution intends to continue running after calling this method,
// it should be called as a goroutine.
// `Lldp` chan carries values of type ndk.LldpNeighborNotification
func (a *Agent) ReceiveLLDPNotifications(ctx context.Context) {
	defer close(a.Notifications.Lldp)
	LldpStream := a.startLldpNotificationStream(ctx)

	for LldpStreamResp := range LldpStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(LldpStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Lldp Neighbor notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received Lldp Neighbor notifications:\n%s", b)

		for _, n := range LldpStreamResp.GetNotification() {
			LldpNotif := n.GetLldpNeighbor()
			if LldpNotif == nil {
				a.logger.Info().
					Msgf("Empty Lldp Neighbor notification:%+v", n)
				continue
			}
			a.Notifications.Lldp <- LldpNotif
		}
	}
}

// startLldpNotificationStream starts a notification stream for Lldp Neighbor service notifications.
func (a *Agent) startLldpNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Lldp Neighbor notification stream created")

	a.addLldpSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"Lldp neighbor", streamChan)

	return streamChan
}

// addLldpSubscription adds a subscription for Lldp Neighbor service notifications
// to the allocated notification stream.
func (a *Agent) addLldpSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for Lldp service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_AddSubscription,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_LldpNeighbor{ // Lldp service
			LldpNeighbor: &ndk.LldpNeighborSubscriptionRequest{},
		},
	}

	registerResp, err := a.SDKMgrServiceClient.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}
