package bond

import (
	"context"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

// ReceiveRouteNotifications starts an route notification stream
// and sends notifications to channel `Route`.
// If the main execution intends to continue running after calling this method,
// it should be called as a goroutine.
// `Route` chan carries values of type ndk.IpRouteNotification
func (a *Agent) ReceiveRouteNotifications(ctx context.Context) {
	defer close(a.Notifs.Route)
	routeStream := a.startRouteNotificationStream(ctx)

	for routeStreamResp := range routeStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(routeStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Route notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received notifications:\n%s", b)

		for _, n := range routeStreamResp.GetNotification() {
			routeNotif := n.GetRoute()
			if routeNotif == nil {
				a.logger.Info().
					Msgf("Empty route notification:%+v", n)
				continue
			}
			a.Notifs.Route <- routeNotif
		}
	}
}

// startRouteNotificationStream starts a notification stream for Route service notifications.
func (a *Agent) startRouteNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Notification stream created")

	a.addRouteSubscription(ctx, streamID)

	streamChan := make(chan *ndk.NotificationStreamResponse)
	go a.startNotificationStream(ctx, streamID,
		"route", streamChan)

	return streamChan
}

// addRouteSubscription adds a subscription for Route service notifications
// to the allocated notification stream.
func (a *Agent) addRouteSubscription(ctx context.Context, streamID uint64) {
	// create notification register request for Route service
	// using acquired stream ID
	notificationRegisterReq := &ndk.NotificationRegisterRequest{
		Op:       ndk.NotificationRegisterRequest_AddSubscription,
		StreamId: streamID,
		SubscriptionTypes: &ndk.NotificationRegisterRequest_Route{ // route service
			Route: &ndk.IpRouteSubscriptionRequest{},
		},
	}

	registerResp, err := a.SDKMgrServiceClient.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}
