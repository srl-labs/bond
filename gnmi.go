package bond

import (
	"errors"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api"
)

const (
	defaultGrpcServerName      = "insecure-mgmt"                               // grpc-server insecure-mgmt
	grpcServerUnixSocketPrefix = "unix:///opt/srlinux/var/run/sr_grpc_server_" // append with grpc-server name
)

var ErrorEmptyValue = errors.New("value to set request cannot be empty")

func (a *Agent) newGNMITarget() error {
	a.logger.Debug().Msg("creating gNMI Client")
	grpcServerUnixSocket := grpcServerUnixSocketPrefix + a.grpcServerName
	// create a target
	target, err := api.NewTarget(
		api.Name("ndk"),
		api.Address(grpcServerUnixSocket),
		api.Username(defaultUsername),
		api.Password(defaultPassword),
		api.Insecure(true),
		api.Timeout(10*time.Second),
	)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("gNMI target creation failed")
		return err
	}

	a.GnmiTarget = target

	err = a.GnmiTarget.CreateGNMIClient(a.ctx)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("gNMI Client creation failed")
	}

	a.logger.Debug().Msg("gNMI Client created")

	return err
}

// NewGetRequest creates a new *gnmi.GetRequest
// using the provided gNMI path and a GNMIOption list opts.
// The list of possible GNMIOption(s) can be imported
// from gnmic api package github.com/openconfig/gnmic/pkg/api.
// An error is returned in case one of the options is invalid
// or if gNMI encoding type is not set (e.g. api.EncodingPROTO, api.EncodingJSON).
func NewGetRequest(path string, opts ...api.GNMIOption) (*gnmi.GetRequest, error) {
	// create a GetRequest
	opts = append(opts, api.Path(path))
	req, err := api.NewGetRequest(opts...)
	return req, err
}

// NewSetUpdateRequest creates a new *gnmi.SetRequest
// that updates the provided gNMI path with the provided value.
// An update value must be provided and can be
// created with api.Value(..).
// A GNMIOption list opts can be as set as well.
// The list of possible GNMIOption(s) can be imported
// from gnmic api package github.com/openconfig/gnmic/pkg/api.
//
// For example: To update value at path /greeter/action-leaf-node with value "delete",
// NewSetUpdateRequest("/greeter/action-leaf-node", api.Value("delete", "string"))
func NewSetUpdateRequest(path string, value api.GNMIOption, opts ...api.GNMIOption) (*gnmi.SetRequest, error) {
	if value == nil {
		return nil, ErrorEmptyValue
	}
	updateReq := api.Update(api.Path(path), value)
	// create a SetRequest
	opts = append(opts, updateReq)
	req, err := api.NewSetRequest(opts...)
	return req, err
}

// NewSetReplaceRequest creates a new *gnmi.SetRequest
// that replaces the provided gNMI path with the provided value.
// A replace value must be provided and can be
// created with api.Value(..).
// A GNMIOption list opts can be as set as well.
// The list of possible GNMIOption(s) can be imported
// from gnmic api package github.com/openconfig/gnmic/pkg/api.
//
// For example: To replace value at path /greeter/action-leaf-node with value "delete",
// NewSetReplaceRequest("/greeter/action-leaf-node", api.Value("delete", "string"))
func NewSetReplaceRequest(path string, value api.GNMIOption, opts ...api.GNMIOption) (*gnmi.SetRequest, error) {
	if value == nil {
		return nil, ErrorEmptyValue
	}
	replaceReq := api.Replace(api.Path(path), value)
	// create a SetRequest
	opts = append(opts, replaceReq)
	req, err := api.NewSetRequest(opts...)
	return req, err
}

// NewSetDeleteRequest creates a new *gnmi.SetRequest
// that deletes all values from the provided gNMI path.
// A GNMIOption list opts can be as set as well.
// The list of possible GNMIOption(s) can be imported
// from gnmic api package github.com/openconfig/gnmic/pkg/api.
//
// For example: To delete values from path /greeter/action-leaf-node,
// NewSetDeleteRequest("/greeter/action-leaf-node")
func NewSetDeleteRequest(path string, opts ...api.GNMIOption) (*gnmi.SetRequest, error) {
	deleteReq := api.Delete(path)
	// create a SetRequest
	opts = append(opts, deleteReq)
	req, err := api.NewSetRequest(opts...)
	return req, err
}

// GetWithGNMI sends a gnmi.GetRequest and returns a gnmi.GetResponse and an error.
// To create a gNMI GetRequest, please use NewGetRequest method.
func (a *Agent) GetWithGNMI(req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	resp, err := a.GnmiTarget.Get(a.ctx, req)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("failed executing GetRequest")
	}

	a.logger.Debug().Msgf("gNMI Get response: %+v", resp)
	return resp, err
}

// SetWithGNMI sends a gnmi.SetRequest and returns a gnmi.SetResponse and an error.
// To create a gNMI SetRequest, consider using NewSet<Update,Replace,Delete>Request methods.
func (a *Agent) SetWithGNMI(req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	resp, err := a.GnmiTarget.Set(a.ctx, req)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("failed executing SetRequest")
	}

	a.logger.Debug().Msgf("gNMI Set response: %+v", resp)
	return resp, err
}

// getConfigWithGNMI gets the config from the gNMI server for the appRootPath
// and stores it in the agent struct.
// gNMI Get Request returns the config in the json_ietf encoding.
// The received config is meant to be used by the NDK app to populate its Config and State struct.
func (a *Agent) getConfigWithGNMI() {
	a.logger.Info().
		Str("path", a.appRootPath).
		Msg("Getting config with gNMI")

	// reset the config as it might contain the previous config
	// and in case we receive an empty config (when config was deleted),
	// we want our FullConfig to be nil
	a.Notifications.FullConfig = nil

	// create a GetRequest
	getReq, err := api.NewGetRequest(
		api.Path(a.appRootPath),
		api.EncodingJSON_IETF(),
		api.DataTypeCONFIG(),
	)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("failed to create GetRequest")
	}

	getResp, err := a.GetWithGNMI(getReq)
	if err != nil {
		return
	}

	// log the received full config if it is not empty
	if len(getResp.GetNotification()) != 0 && len(getResp.GetNotification()[0].GetUpdate()) != 0 {
		a.Notifications.FullConfig = getResp.GetNotification()[0].
			GetUpdate()[0].
			GetVal().
			GetJsonIetfVal()

		a.logger.Info().Msgf("Full config received via gNMI:\n%s", a.Notifications.FullConfig)
	}
}
