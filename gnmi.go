package bond

import (
	"time"

	"github.com/openconfig/gnmic/pkg/api"
)

const (
	grpcServerUnixSocket = "unix:///opt/srlinux/var/run/sr_grpc_server_insecure-mgmt" // grpc-server insecure-mgmt
	jsonIETFEncoding     = "json_ietf"
)

func (a *Agent) newGNMITarget() error {
	a.logger.Debug().Msg("creating gNMI Client")

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

	a.gNMITarget = target

	a.logger.Debug().Msg("gNMI Client created")

	return err
}

// getConfigWithGNMI gets the config from the gNMI server for the appRootPath
// and stores it in the agent struct.
// gNMI Get Request returns the config in the json_ietf encoding.
// The received config is meant to be used by the NDK app to populate its Config and State struct.
func (a *Agent) getConfigWithGNMI() {
	a.logger.Info().
		Str("root-path", a.appRootPath).
		Msg("Getting config with gNMI")

	// reset the config as it might contain the previous config
	// and in case we receive an empty config (when config was deleted),
	// we want our Config to be nil
	a.Notifications.Config = nil

	err := a.gNMITarget.CreateGNMIClient(a.ctx)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("gNMI client failed")
	}
	defer a.gNMITarget.Close()

	// create a GetRequest
	getReq, err := api.NewGetRequest(
		api.Path(a.appRootPath),
		api.Encoding(jsonIETFEncoding),
		api.DataTypeCONFIG(),
	)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("failed to create GetRequest")
	}

	getResp, err := a.gNMITarget.Get(a.ctx, getReq)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("failed executing GetRequest")
	}

	a.logger.Debug().Msgf("gNMI Get response: %+v", getResp)

	// log the received config if it is not empty
	if len(getResp.GetNotification()) != 0 && len(getResp.GetNotification()[0].GetUpdate()) != 0 {
		a.Notifications.Config = getResp.GetNotification()[0].
			GetUpdate()[0].
			GetVal().
			GetJsonIetfVal()

		a.logger.Info().Msgf("Config received via gNMI:\n%s", a.Notifications.Config)
	}
}
