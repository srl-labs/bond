package bond

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

var ErrInvalidIpAddr = errors.New("invalid ip address provided")
var ErrRouteDeleteFailed = errors.New("route delete failed")
var ErrRouteAddOrUpdateFailed = errors.New("route add or update failed")
var ErrRouteSyncStart = errors.New("route sync start failed")
var ErrRouteSyncEnd = errors.New("route sync end failed")

// Options when adding/updating IP routes.
type RouteOption func(r *ndk.RouteInfo)

// New creates a NDK route with the provided route option fields.
// A valid route requires the following option fields:
// WithNetInstName, WithIpPrefix, and WithNextHopGroup.
// Optional: WithPreference, WithMetric
func NewRoute(options ...RouteOption) *ndk.RouteInfo {
	r := new(ndk.RouteInfo)
	r.Data = new(ndk.RoutePb)
	r.Key = new(ndk.RouteKeyPb)
	r.Key.IpPrefix = new(ndk.IpAddrPrefLenPb)
	for _, opt := range options {
		opt(r)
	}
	return r
}

// WithNetInstName sets the route network instance name.
//
// Example: default
func WithNetInstName(n string) RouteOption {
	return func(r *ndk.RouteInfo) {
		r.Key.NetInstName = n
	}
}

// WithIpPrefix sets the route ipv4 or ipv6 prefix.
// prefix string is in the format of  "ip/preflen"
// where ip is the IP address and preflen is the length of the prefix.
// If the input string does not match the expected format,
// RouteAdd/Update returns an error.
//
// Example: 192.168.11.2/30
func WithIpPrefix(prefix string) RouteOption {
	return func(r *ndk.RouteInfo) {
		addr, preflen := parseIP(prefix)
		r.Key.IpPrefix = &ndk.IpAddrPrefLenPb{
			IpAddr:       addr,
			PrefixLength: preflen,
		}
	}
}

// WithNextHopGroupName sets the route Next Hop Group Name.
// NDK expects the input nhg to end in the format "_sdk" or "_SDK".
// If the input string does not match the expected format,
// RouteAdd returns an error.
// Specified nhg also must be a valid NDK next hop group that is programmed
// with method NextHopGroupAdd or NextHopGroupUpdate.
// It cannot be a nexthop group configured on SRL.
//
// Example: ndk_sdk
func WithNextHopGroupName(nhg string) RouteOption {
	return func(r *ndk.RouteInfo) {
		r.Data.NexthopGroupName = nhg
	}
}

// WithMetric sets the route metric value.
func WithMetric(m uint32) RouteOption {
	return func(r *ndk.RouteInfo) {
		r.Data.Metric = m
	}
}

// WithPreference sets the route preference value.
func WithPreference(p uint32) RouteOption {
	return func(r *ndk.RouteInfo) {
		r.Data.Preference = p
	}
}

// RouteAdd adds agent IP route(s) in SR Linux.
// This method takes route(s) of type RouteInfo,
// which is defined in the NDK Go Bindings.
// RouteInfo struct(s) can be populated by method NewRoute
// with options configured using the With<route_field> functions.
// Options that need to be included are ip prefix,
// network instance name,and next hop group name.
// If errors are encountered during the parsing of prefixes or
// adding of routes, an error is returned.
func (a *Agent) RouteAdd(routes ...*ndk.RouteInfo) error {
	infos := []*ndk.RouteInfo{}
	infos = append(infos, routes...)
	req := &ndk.RouteAddRequest{
		Routes: infos,
	}

	// call NDK RPC
	a.logger.Info().Msg("Add/Update routes")
	resp, err := a.stubs.routeService.RouteAddOrUpdate(a.ctx, req)
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failed to add/update routes, response: %v", resp)
		return fmt.Errorf("%w", ErrRouteAddOrUpdateFailed)
	}
	a.logger.Debug().
		Msgf("Successfully added/updated routes, response: %v", resp)
	return nil
}

// RouteUpdate updates and performs resynchronization on programmed NDK routes.
// Routes not added as part of this update are removed from FIB.
// Routes added as part of this update are added to the FIB.
// This method takes route(s) of type RouteInfo,
// which is defined in the NDK Go Bindings.
// RouteInfo struct(s) can be populated by method NewRoute
// with options configured using the With<route_field> functions.
// Options that need to be included are ip prefix,
// network instance name, and next hop group name.
// If errors are encountered during the parsing of prefixes or
// adding of routes, an error is returned.
//
// Example:
// If routes 1.1.1.1, 1.1.1.2 were previously added to FIB with RouteAdd,
// RouteUpdate with routes 1.1.1.1, 1.1.1.3 will result in
// FIB with routes 1.1.1.1, 1.1.1.3.
// Route 1.1.1.2 that was previously added, is deleted due to the update.
func (a *Agent) RouteUpdate(routes ...*ndk.RouteInfo) error {
	err := a.routeSyncStart()
	if err != nil {
		return err
	}
	err = a.RouteAdd(routes...)
	if err != nil {
		return err
	}
	err = a.routeSyncEnd()
	if err != nil {
		return err
	}
	return nil
}

// RouteDelete deletes agent IP route(s) in SR Linux.
// The method takes single or multiple IPv4/IPv6 prefixes
// under a network instance name (e.g. default).
// prefixes is a string in the format of  "ip/preflen"
// where ip is the IP address and preflen is the length of the prefix.
// If errors are encountered during the parsing of prefixes or
// deleting of routes, an error is returned.
//
// Example: RouteDelete("default", "192.168.11.1/24") deletes from FIB
// an IPv4 address with a prefix length of 24.
// Example: RouteDelete("default", "2001:db8::1/64") deletes from FIB
// an IPv6 address with a prefix length of 64.
func (a *Agent) RouteDelete(networkInstance string, prefixes ...string) error {
	keys := []*ndk.RouteKeyPb{}
	for _, prefix := range prefixes {
		addr, preflen := parseIP(prefix)
		if addr == nil || preflen == 0 {
			a.logger.Error().
				Msgf("Invalid IP prefix %s.", addr)
			return fmt.Errorf("%w", ErrInvalidIpAddr)
		}
		prefix := &ndk.IpAddrPrefLenPb{
			IpAddr:       addr,
			PrefixLength: preflen,
		}
		key := &ndk.RouteKeyPb{
			NetInstName: networkInstance,
			IpPrefix:    prefix,
		}
		keys = append(keys, key)
	}
	req := &ndk.RouteDeleteRequest{
		Routes: keys,
	}

	// call NDK RPC
	a.logger.Info().Msg("Delete routes")
	resp, err := a.stubs.routeService.RouteDelete(a.ctx, req)
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failed to delete routes, response: %v", resp)
		return fmt.Errorf("%w", ErrRouteDeleteFailed)
	}
	a.logger.Debug().
		Msgf("Successfully deleted routes, response: %v", resp)
	return nil
}

// routeSyncStart starts syncing agent IP routes in SR Linux.
func (a *Agent) routeSyncStart() error {
	resp, err := a.stubs.routeService.SyncStart(a.ctx, &ndk.SyncRequest{})
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failure to start syncing routes, response: %v", resp)
		return fmt.Errorf("%w", ErrRouteSyncStart)
	}
	a.logger.Debug().
		Msgf("Successfully started route sync, response: %v", resp)
	return nil
}

// routeSyncEnd ends syncing agent IP routes in SR Linux.
func (a *Agent) routeSyncEnd() error {
	resp, err := a.stubs.routeService.SyncEnd(a.ctx, &ndk.SyncRequest{})
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failure to stop syncing routes, response: %v", resp)
		return fmt.Errorf("%w", ErrRouteSyncEnd)
	}
	a.logger.Debug().
		Msgf("Successfully ended route sync, response: %v", resp)
	return nil
}

// parseIP takes an IPv4/IPv6 prefix, then splits it by address and prefix length.
func parseIP(ip string) (address *ndk.IpAddressPb, preflen uint32) {
	var l int
	// split an ip address by "addr/len"
	ret := strings.Split(ip, "/")
	addr := ret[0]
	// convert the string ip addr to bytes
	bytes := net.ParseIP(addr)
	if bytes != nil {
		if bytes.To4() != nil { // is ipv4 addr
			bytes = bytes.To4()
		}
		address := &ndk.IpAddressPb{
			Addr: bytes,
		}

		if len(ret) == 1 {
			return address, 0
		}
		l, _ = strconv.Atoi(ret[1])
		return address, uint32(l)
	}
	return nil, 0
}
