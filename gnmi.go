package bond

import (
	"github.com/openconfig/gnmic/pkg/api"
)

const (
	grpcServerUnixSocket = "unix:///opt/srlinux/var/run/sr_gnmi_server"
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
	)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("gNMI target creation failed")
		return err
	}

	a.gNMITarget = target

	a.logger.Debug().Msg("gNMI Client created")

	return err
}

func (a *Agent) getConfigWithGNMI() {
	a.logger.Info().
		Str("root-path", a.appRootPath).
		Msg("Getting config with gNMI")

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
	if len(getResp.GetNotification()) == 0 {
		a.logger.Info().Msgf("Config:\n%s", getResp.GetNotification()[0].
			GetUpdate()[0].
			GetVal().
			GetJsonIetfVal())
	}
}
