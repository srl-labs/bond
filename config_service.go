package bond

import (
	"errors"
	"fmt"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

var (
	ErrAckCfgFailed       = errors.New("acknowledge config failed")
	ErrAckCfgOptionNotSet = errors.New("agent is not registered with WaitAckConfig option")
)

type Acknowledgement = ndk.AcknowledgeConfigRequestInfo
type Message func(n *Acknowledgement)

// NewAcknowledgement creates a config Acknowledgement.
// - An acknowledgement contains a path
// in XPath format which targets an app's
// config YANG schema node.
// All the possible YANG path targets are
// listed below with it's corresponding example.
// - container: /greeter, /greeter/container-node
// - leaf: /greeter/leaf-node
// - leaf-list: /greeter/leaf-list-node[leaf-list-node=*]
// - leaf-list entry: /greeter/leaf-list-node[leaf-list-node=<entry>]
// - list: /greeter/list-node[name=*]
// - list entry: /greeter/list-node[name=<entry>]
// A YANG leaf-list and list entry can be targeted
// individually with it's <entry> name,
// instead of using the wildcard '*' character.
// - An Acknowledgement message m can also be passed in.
// After a config is acked back to SR Linux, the message
// m will be reflected in the CLI during commit phase.
// A message type can either be an output, warning, or error
// which can be created using the functions (Output, Error, Warning).
// If either path or message is not provided,
// an acknowledgement with empty contents will be returned.
func NewAcknowledgement(path string, m Message) *Acknowledgement {
	a := new(ndk.AcknowledgeConfigRequestInfo)
	if path == "" || m == nil {
		return a
	}
	a.JsPathWithKeys = convertXPathToJSPath(path)
	m(a)
	return a
}

// Output returns an output Message, given the string o.
func Output(o string) Message {
	return func(a *Acknowledgement) {
		a.Result = &ndk.AcknowledgeConfigRequestInfo_Output{Output: o}
	}
}

// Warning returns a warning Message, given the string w.
func Warning(w string) Message {
	return func(a *Acknowledgement) {
		a.Result = &ndk.AcknowledgeConfigRequestInfo_Warning{Warning: w}
	}
}

// Error returns an error Message, given the string e.
func Error(e string) Message {
	return func(a *Acknowledgement) {
		a.Result = &ndk.AcknowledgeConfigRequestInfo_Error{Error: e}
	}
}

// AcknowledgeConfig explicitly acknowledges configs with SR Linux.
// - If Agent has WithConfigAcknowledge option set, SR Linux
// will wait for explicit ack from app before commit
// completing the configuration.
// Note: an error is returned if app does not have
// both streaming of configs (WithStreamConfig)
// and WithConfigAcknowledge options enabled.
// - This method should only be called once
// for all configs in a commit (.commit.end).
// If app calls AcknowledgeConfig multiple times for a commit,
// any calls after the first one will be ignored by ndk server.
// - `acks` can contain one or multiple Acknowledgement,
// each targeting an app YANG config node
// with a corresponding message.
// During commit phase, SR Linux will reflect in
// the CLI all ack messages and their corresponding paths.
// If a particular ack contains an Error message,
// the entire commit is rejected during commit phase.
// SR Linux will then rollback the commit to the
// app's previous valid running configuration.
// During rollback, NDK server will then stream to app
// the valid config notifications.
// If `acks` is empty, SR Linux will still treat this as
// a valid acknowledgement, but with empty data.
func (a *Agent) AcknowledgeConfig(acks ...*Acknowledgement) error {
	if !a.configAck {
		a.logger.Error().
			Msgf(`Agent cannot AcknowledgeConfig if 
				WithConfigAcknowledge option is not enabled.`)
		return fmt.Errorf("%w", ErrAckCfgOptionNotSet)
	}
	if !a.streamConfig {
		a.logger.Error().
			Msgf(`Agent cannot AcknowledgeConfig if
				streaming of configs is not enabled.`)
		return fmt.Errorf("%w", ErrAckCfgAndNotStreamCfg)
	}
	infos := []*ndk.AcknowledgeConfigRequestInfo{}
	infos = append(infos, acks...)
	req := &ndk.AcknowledgeConfigRequest{
		Infos: infos,
	}
	// Call NDK RPC
	a.logger.Info().Msgf("Acknowledge Config %v with NDK server", req)
	resp, err := a.stubs.configService.AcknowledgeConfig(a.ctx, req)
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failed to acknowledge config, response: %v", resp)
		return fmt.Errorf("%w", ErrAckCfgFailed)
	}
	a.logger.Debug().
		Msgf("Agent was able to acknowledge config, response: %v", resp)
	return nil
}
