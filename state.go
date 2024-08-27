package bond

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

var ErrStateDeleteFailed = errors.New("state delete failed")
var ErrStateAddOrUpdateFailed = errors.New("state add/update failed")

// DeleteAllState completely deletes the state of an application.
// All state added with UpdateState will be deleted.
func (a *Agent) DeleteAllState() error {
	for path := range a.paths {
		err := a.DeleteState(path)
		if err != nil {
			return err
		}
	}
	a.paths = make(map[string]struct{}) // reinitialize cache
	return nil
}

// DeleteState deletes application's state for a YANG list entry or the root container.
// It takes in a path which follows XPath format.
// Examples include /greeter, the app's root container or
// /greeter/list-node[name=entry1], a list entry of `list-node`.
// If no path is provided, the app's root container is assumed by default.
func (a *Agent) DeleteState(path string) error {
	var jsPath string

	a.logger.Info().
		Str("path", path).
		Msg("Deleting state")

	if path == "" {
		jsPath = strings.ReplaceAll(a.appRootPath, "/", ".")
	} else {
		jsPath = convertXPathToJSPath(path)
	}

	// verify state for path was added previously
	_, ok := a.paths[path]
	if !ok {
		a.logger.Error().
			Msgf("Trying to delete state for path %s that has never been added.", path)
		return fmt.Errorf("%w: path: %s", ErrStateDeleteFailed, path)
	}

	key := &ndk.TelemetryKey{JsPath: jsPath}

	r, err := a.stubs.telemetryService.TelemetryDelete(a.ctx, &ndk.TelemetryDeleteRequest{
		Key: []*ndk.TelemetryKey{key},
	})
	if err != nil || r.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().Msgf("Failed to delete state, response: %v", r)
		return fmt.Errorf("%w: path: %s", ErrStateDeleteFailed, jsPath)
	}
	delete(a.paths, path)
	return nil
}

// UpdateState updates application's state for a YANG list entry or the root container.
// It takes in a path which follows XPath format.
// Examples include /greeter, the app's root container or
// /greeter/list-node[name=entry1], a list entry of `list-node`.
// data is the target path's json state, which may contain leaf or leaf-list json data.
// State for paths added with UpdateState may be deleted with DeleteState or DeleteAllState.
func (a *Agent) UpdateState(path, data string) error {
	var jsPath string

	a.logger.Info().
		Str("path", path).
		Str("data", data).
		Msg("Updating state")

	if path == "" {
		jsPath = strings.ReplaceAll(a.appRootPath, "/", ".")
	} else {
		jsPath = convertXPathToJSPath(path)
	}

	tkey := &ndk.TelemetryKey{JsPath: jsPath}
	tdata := &ndk.TelemetryData{JsonContent: data}
	info := &ndk.TelemetryInfo{Key: tkey, Data: tdata}
	req := &ndk.TelemetryUpdateRequest{
		State: []*ndk.TelemetryInfo{info},
	}

	a.logger.Info().Msgf("Telemetry Request: %+v", req)

	r, err := a.stubs.telemetryService.TelemetryAddOrUpdate(a.ctx, req)
	if err != nil || r.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		return fmt.Errorf("%w: key: %s, data: %s", ErrStateAddOrUpdateFailed, jsPath, data)
	}
	a.paths[path] = struct{}{} // add path to cache
	return nil
}
