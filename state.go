package bond

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

var ErrStateDeleteFailed = errors.New("state delete failed")
var ErrStateAddOrUpdateFailed = errors.New("state add/update failed")

// DeleteState deletes application's state for a YANG list entry or the root container.
// It takes in a target path which follows XPath format.
// Possible YANG path targets are the app's root container (e.g. /greeter) or
// a YANG list entry (e.g. /greeter/list-node[name=entry1]).
// All state for child schema nodes will be deleted.
// If empty path is provided, the app's root container is assumed by default
// and the entire application state is deleted.
func (a *Agent) DeleteState(path string) error {
	a.logger.Info().
		Str("path", path).
		Msg("Deleting state")

	// delete all app state
	var deleteAll bool // optimize by avoiding strings.HasPrefix
	if path == "" || path == a.appRootPath {
		path = a.appRootPath
		deleteAll = true
	}

	// verify state for path was added previously
	_, ok := a.paths[path]
	if !ok {
		a.logger.Error().
			Msgf("Trying to delete state for path %s that has never been added.", path)
		return fmt.Errorf("%w: path: %s", ErrStateDeleteFailed, path)
	}

	deleteOk := true // indicates whether to delete path
	for p := range a.paths {
		if !deleteAll {
			deleteOk = strings.HasPrefix(p, path) // delete child?
		}
		if !deleteOk {
			continue
		}

		jsPath := convertXPathToJSPath(p)
		key := &ndk.TelemetryKey{JsPath: jsPath}

		r, err := a.stubs.telemetryService.TelemetryDelete(a.ctx, &ndk.TelemetryDeleteRequest{
			Keys: []*ndk.TelemetryKey{key},
		})
		if err != nil || r.GetStatus() != ndk.SdkMgrStatus_SDK_MGR_STATUS_SUCCESS {
			a.logger.Error().Msgf("Failed to delete state, response: %v", r)
			return fmt.Errorf("%w: path: %s", ErrStateDeleteFailed, jsPath)
		}
		delete(a.paths, p)
		deleteOk = true
	}
	return nil
}

// UpdateState updates application's state for a YANG list entry or the root container.
// It takes in a path which follows XPath format.
// Examples include /greeter, the app's root container or
// /greeter/list-node[name=entry1], a list entry of `list-node`.
// data is the target path's json state, which may contain leaf or leaf-list json data.
// State for paths added with UpdateState may be deleted with DeleteState.
func (a *Agent) UpdateState(path, data string) error {
	var jsPath string

	a.logger.Info().
		Str("path", path).
		Str("data", data).
		Msg("Updating state")

	if path == "" {
		path = a.appRootPath
		jsPath = strings.ReplaceAll(path, "/", ".")
	} else {
		jsPath = convertXPathToJSPath(path)
	}

	tkey := &ndk.TelemetryKey{JsPath: jsPath}
	tdata := &ndk.TelemetryData{JsonContent: data}
	info := &ndk.TelemetryInfo{Key: tkey, Data: tdata}
	req := &ndk.TelemetryUpdateRequest{
		States: []*ndk.TelemetryInfo{info},
	}

	a.logger.Info().Msgf("Telemetry Request: %+v", req)

	r, err := a.stubs.telemetryService.TelemetryAddOrUpdate(a.ctx, req)
	if err != nil || r.GetStatus() != ndk.SdkMgrStatus_SDK_MGR_STATUS_SUCCESS {
		return fmt.Errorf("%w: key: %s, data: %s", ErrStateAddOrUpdateFailed, jsPath, data)
	}
	a.paths[path] = struct{}{} // add path to cache
	return nil
}
