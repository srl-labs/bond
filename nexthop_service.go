package bond

import (
	"errors"
	"fmt"

	"github.com/nokia/srlinux-ndk-go/ndk"
)

var ErrNhgAddOrUpdateFailed = errors.New("nexthop group add or update failed")
var ErrNhgDeleteFailed = errors.New("nexthop group delete failed")
var ErrNhgSyncStart = errors.New("nexthop group start failed")
var ErrNhgSyncEnd = errors.New("nexthop group sync end failed")

// Options when adding/updating nexthop groups.
type NextHopGroupOption func(n *ndk.NextHopGroupInfo)

// NewNextHopGroup creates a NDK nexthop group
// with the provided option fields.
// A valid nexthop group requires the following options:
// WithNetworkInstanceName, WithName, WithIpNextHop or WithMplsNextHop
// Multiple nexthop(s) can be associated with a nexthop group.
func NewNextHopGroup(options ...NextHopGroupOption) *ndk.NextHopGroupInfo {
	n := new(ndk.NextHopGroupInfo)
	n.Key = new(ndk.NextHopGroupKey)
	n.Data = new(ndk.NextHopGroup)
	nhops := []*ndk.NextHop{}
	n.Data.NextHop = nhops
	for _, opt := range options {
		opt(n)
	}
	return n
}

// WithNetworkInstanceName sets the nexthop group network instance name.
//
// Example: default
func WithNetworkInstanceName(name string) NextHopGroupOption {
	return func(n *ndk.NextHopGroupInfo) {
		n.Key.NetworkInstanceName = name
	}
}

// WithName sets the nexthop group name.
// NDK expects the input name to end in the format "_sdk" or "_SDK".
// If the input string does not match the expected format,
// NextHopGroupAdd returns an error.
// Specified nhg must be a valid NDK nexthop group that will be programmed
// with method NextHopGroupAdd or NextHopGroupUpdate.
// It cannot be a nexthop group configured on SRL.
//
// Example: ndk_sdk
func WithName(name string) NextHopGroupOption {
	return func(n *ndk.NextHopGroupInfo) {
		n.Key.Name = name
	}
}

// WithIpNextHop adds a IP nexthop to this nexthop group.
// An IP nexthop is defined by it's IPv4/IPv6 address,
// the resolution type, and type of routes it resolves to.
// address string is in the format of  "ip"
// where ip is the IP address without the prefix length.
// rt is of type ndk.NextHop_ResolveToType.
// rType is of type ndk.NextHop_ResolutionType.
// Both of these params are defined in the NDK Go Bindings.
//
// Example:
// WithIpNextHop(1.1.1.1, ndk.NextHop_DIRECT, ndk.NextHop_REGULAR)
func WithIpNextHop(address string, rt ndk.NextHop_ResolveToType, rType ndk.NextHop_ResolutionType) NextHopGroupOption {
	return func(n *ndk.NextHopGroupInfo) {
		nhParse, _ := parseIP(address)
		nh := &ndk.NextHop{
			Nexthop: &ndk.NextHop_IpNexthop{
				IpNexthop: nhParse,
			},
			ResolveTo: rt,
			Type:      rType,
		}
		n.Data.NextHop = append(n.Data.NextHop, nh)
	}
}

// WithMplsNextHop adds a MPLS nexthop to this nexthop group.
// A MPLS nexthop is defined by it's IPv4/IPv6 address,
// a slice of uint32 MPLS labels, the resolution type,
// and type of routes it resolves to.
// address string is in the format of  "ip"
// where ip is the IP address without the prefix length.
// rt is of type ndk.NextHop_ResolveToType.
// rType is of type ndk.NextHop_ResolutionType.
// Both of these params are defined in the NDK Go Bindings.
//
// Example:
// WithMplsNextHop(1.1.1.1, [100], ndk.NextHop_DIRECT, ndk.NextHop_REGULAR)
func WithMplsNextHop(address string, labels []uint32, rt ndk.NextHop_ResolveToType,
	rType ndk.NextHop_ResolutionType) NextHopGroupOption {
	return func(n *ndk.NextHopGroupInfo) {
		nhParse, _ := parseIP(address)
		lStack := []*ndk.MplsLabel{}
		for _, l := range labels {
			lStack = append(lStack, &ndk.MplsLabel{
				MplsLabel: l,
			})
		}
		nh := &ndk.NextHop{
			Nexthop: &ndk.NextHop_MplsNexthop{
				MplsNexthop: &ndk.MplsNextHop{
					IpNexthop:  nhParse,
					LabelStack: lStack,
				},
			},
			ResolveTo: rt,
			Type:      rType,
		}
		n.Data.NextHop = append(n.Data.NextHop, nh)
	}
}

// NextHopGroupAdd adds nexthop group(s) in SRL.
// This method takes nexthop group(s) of type NextHopGroupInfo,
// which is defined in the NDK Go Bindings.
// NextHopGroupInfo struct(s) can be populated by method NewNextHopGroup
// with options configured using the With<nhg_field> functions.
// Options that need to be included are name,
// network instance name, and nexthop addresses.
// If errors are encountered during the parsing of addresses or
// adding of nexthop groups, an error is returned.
func (a *Agent) NextHopGroupAdd(nhgs ...*ndk.NextHopGroupInfo) error {
	infos := []*ndk.NextHopGroupInfo{}
	infos = append(infos, nhgs...)
	req := &ndk.NextHopGroupRequest{
		GroupInfo: infos,
	}
	// Call NDK RPC
	a.logger.Info().Msg("Add/update nexthop(s) group")
	resp, err := a.stubs.nextHopGroupService.NextHopGroupAddOrUpdate(a.ctx, req)
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failed to add or update nexthop groups, response: %v", resp)
		return fmt.Errorf("%w", ErrNhgAddOrUpdateFailed)
	}
	a.logger.Debug().
		Msgf("Agent was able to add or update nexthop group, response: %v", resp)
	return nil
}

// NextHopGroupUpdate updates and performs resynchronization
// on programmed NDK nexthop group(s).
// Nexthop groups not added as part of this update
// are removed from the ephemeral configuration.
// Nexthop groups added as part of this update
// are added to the ephemeral configuration.
// This method takes nexthop group(s) of type NextHopGroupInfo,
// which is defined in the NDK Go Bindings.
// NextHopGroupInfo struct(s) can be populated by method NewNextHopGroup
// with options configured using the With<nhg_field> functions.
// Options that need to be included are name,
// network instance name, and nexthop(s) details.
// If errors are encountered during the parsing of addresses or
// adding of nexthop groups, an error is returned.
//
// Example:
// If nexthop groups with addresses 1.1.1.1, 1.1.1.2 were previously
// programmed with NextHopGroupAdd,
// a NextHopGroupUpdate on nhgs with addresses 1.1.1.2, 1.1.1.3
// will result in the final configuration being 1.1.1.2, 1.1.1.3.
// Nexthop group with address 1.1.1.1, which was previously added, is deleted due to the update.
func (a *Agent) NextHopGroupUpdate(nhgs ...*ndk.NextHopGroupInfo) error {
	err := a.nhgSyncStart()
	if err != nil {
		return err
	}
	err = a.NextHopGroupAdd(nhgs...)
	if err != nil {
		return err
	}
	err = a.nhgSyncEnd()
	if err != nil {
		return err
	}
	return nil
}

// NextHopGroupDelete deletes a programmed nexthop group
// that has been added/updated by the NDK.
// The method takes as inputs the network instance name and the nexthop group name.
// If errors are encountered during the deletion
// of nexthop group, an error is returned.
//
// Example: NextHopGroupDelete("default", "ndk_sdk") deletes from programmed config
// ndk_sdk nexthop group in network instance default.
func (a *Agent) NextHopGroupDelete(networkInstance string, name string) error {
	key := &ndk.NextHopGroupKey{
		Name:                name,
		NetworkInstanceName: networkInstance,
	}
	keys := []*ndk.NextHopGroupKey{
		key,
	}
	req := &ndk.NextHopGroupDeleteRequest{
		GroupKey: keys,
	}
	// Call NDK RPC
	a.logger.Info().Msg("Delete nexthop group")
	resp, err := a.stubs.nextHopGroupService.NextHopGroupDelete(a.ctx, req)
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failed to delete nexthop group, response: %v", resp)
		return fmt.Errorf("%w", ErrNhgDeleteFailed)
	}
	a.logger.Debug().
		Msgf("Agent was able to delete nexthop group, response: %v", resp)
	return nil
}

// nhgSyncStart starts syncing agent nexthop groups in SRL.
func (a *Agent) nhgSyncStart() error {
	resp, err := a.stubs.nextHopGroupService.SyncStart(a.ctx, &ndk.SyncRequest{})
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failure to start syncing nexthop groups, response: %v", resp)
		return fmt.Errorf("%w", ErrNhgSyncStart)
	}
	a.logger.Debug().
		Msgf("Successfully started nexthop group sync, response: %v", resp)
	return nil
}

// nhgSyncEnd ends syncing agent nexthop groups in SRL.
func (a *Agent) nhgSyncEnd() error {
	resp, err := a.stubs.nextHopGroupService.SyncEnd(a.ctx, &ndk.SyncRequest{})
	if err != nil || resp.GetStatus() != ndk.SdkMgrStatus_kSdkMgrSuccess {
		a.logger.Error().
			Msgf("Failure to stop syncing nexthop groups, response: %v", resp)
		return fmt.Errorf("%w", ErrNhgSyncEnd)
	}
	a.logger.Debug().
		Msgf("Successfully stopped nexthop group sync, response: %v", resp)
	return nil
}
