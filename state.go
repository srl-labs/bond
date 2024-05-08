package bond

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

var ErrStateDeleteFailed = errors.New("state delete failed")
var ErrStateAddOrUpdateFailed = errors.New("state add/update failed")

// DeleteState completely deletes the state of an app.
func (a *Agent) DeleteState() error {
	rootJSPath := strings.ReplaceAll(a.appRootPath, "/", ".")

	a.logger.Info().
		Str("path", rootJSPath).
		Msg("Deleting state")

	key := &ndk.TelemetryKey{JsPath: rootJSPath}

	r, err := a.stubs.telemetryService.TelemetryDelete(a.ctx, &ndk.TelemetryDeleteRequest{
		Key: []*ndk.TelemetryKey{key},
	})
	if err != nil || r.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().Msgf("Failed to delete state, response: %v", r)
		return fmt.Errorf("%w: path: %s", ErrStateDeleteFailed, a.appRootPath)
	}

	return nil
}

// UpdateState updates application's state by the given key with the data.
func (a *Agent) UpdateState(key, data string) error {
	if key == "" {
		key = strings.ReplaceAll(a.appRootPath, "/", ".")
	}

	a.logger.Info().
		Str("key", key).
		Str("data", data).
		Msg("Updating state")

	tkey := &ndk.TelemetryKey{JsPath: key}
	tdata := &ndk.TelemetryData{JsonContent: data}
	info := &ndk.TelemetryInfo{Key: tkey, Data: tdata}
	req := &ndk.TelemetryUpdateRequest{
		State: []*ndk.TelemetryInfo{info},
	}

	a.logger.Info().Msgf("Telemetry Request: %+v", req)

	r, err := a.stubs.telemetryService.TelemetryAddOrUpdate(a.ctx, req)
	if err != nil || r.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		return fmt.Errorf("%w: key: %s, data: %s", ErrStateAddOrUpdateFailed, key, data)
	}

	return nil
}
