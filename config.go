package bond

import (
	"context"
	"encoding/json"

	"github.com/nokia/srlinux-ndk-go/ndk"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	commitEndKeyPath = ".commit.end"
)

// ConfigNotification type defines streamed notification contents.
// Possible Path targets are the app's
// root YANG container or any list entries.
// e.g. /greeter, /greeter/list-node[name=entry1]
// When all configs for a particular commit have been streamed,
// app will receive a Config notification with Path .commit.end.
// PathWithoutKeys is the Path but wihout the list keys.
// For example, a Path of /greeter/list-a[name=entry1]/list-b
// would have PathWithoutKeys of /greeter/list-a/list-b.
// Possible Op values are Create, Update, or Delete.
// For example: if a config is deleted, the notification
// would have an Op Delete.
// Json contains leaf, leaf-list, or child container
// configs for the target Path.
type ConfigNotification struct {
	Op              string   // NDK config operation
	Path            string   // YANG path that follows XPath format
	PathWithoutKeys string   // YANG path without list keys
	Keys            []string // Value for keys, only returned for YANG list configs
	Json            string   // Entire configuration fragment as JSON string
}

// receiveConfigNotifications receives a stream of configuration notifications
// buffer them in the configuration buffer and populates ConfigState struct of the App
// once the whole committed config is received.
func (a *Agent) receiveConfigNotifications(ctx context.Context) {
	configStream := a.startConfigNotificationStream(ctx)

	for cfgStreamResp := range configStream {
		b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(cfgStreamResp)
		if err != nil {
			a.logger.Info().
				Msgf("Config notification Marshal failed: %+v", err)
			continue
		}

		a.logger.Info().
			Msgf("Received Config notifications:\n%s", b)

		a.handleConfigNotifications(cfgStreamResp)
	}
}

// startConfigNotificationStream starts a notification stream for Config service notifications.
func (a *Agent) startConfigNotificationStream(ctx context.Context) chan *ndk.NotificationStreamResponse {
	streamID := a.createNotificationStream(ctx)

	a.logger.Info().
		Uint64("stream-id", streamID).
		Msg("Config notification stream created")

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

	registerResp, err := a.stubs.sdkMgrService.NotificationRegister(ctx, notificationRegisterReq)
	if err != nil || registerResp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Printf("agent %s failed registering to notification with req=%+v: %v",
			a.Name, notificationRegisterReq, err)
	}
}

// handleConfigNotifications logs configuration notifications received
// from the config notification stream and signals the
// FullConfigReceived chan when the full config is received.
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
		if !a.streamConfig {
			if cfgNotif.Key.JsPath == commitEndKeyPath &&
				!a.isCommitSeqZero(cfgNotif.GetData().GetJson()) {
				a.logger.Debug().
					Msgf("Received commit end notification: %+v", cfgNotif)

				a.getConfigWithGNMI()

				a.Notifications.FullConfigReceived <- struct{}{}
			}
		} else { // stream configs individually
			a.Notifications.Config <- parseConfig(cfgNotif)
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

// parseConfig parses a streamed config notification and returns the contents ConfigNotification.
// Most of the post-processing involves converting NDK JsPaths to YANG XPaths.
func parseConfig(n *ndk.ConfigNotification) *ConfigNotification {
	if n == nil {
		return nil
	}
	cfg := new(ConfigNotification)
	cfg.Op, cfg.Json = n.GetOp().String(), n.GetData().GetJson()
	cfg.Keys = n.GetKey().GetKeys()
	cfg.Path = n.GetKey().GetJsPathWithKeys()
	cfg.PathWithoutKeys = n.GetKey().GetJsPath()
	if cfg.Path == ".commit.end" { // don't convert commit end path
		return cfg
	}
	cfg.Path = convertJSPathToXPath(cfg.Path)
	cfg.PathWithoutKeys = convertJSPathToXPath(cfg.PathWithoutKeys)
	return cfg
}
