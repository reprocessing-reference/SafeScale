package providers

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/lib/server/iaas/stacks"
	"github.com/CS-SI/SafeScale/lib/server/iaas/userdata"
	"github.com/CS-SI/SafeScale/lib/server/resources/abstract"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/hoststate"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
	"github.com/CS-SI/SafeScale/lib/utils/temporal"
)

// LoggedProvider ...
type LoggedProvider WrappedProvider

func (w LoggedProvider) ListSecurityGroups() ([]*abstract.SecurityGroup, fail.Error) {
	defer w.prepare(w.trace("ListSecurityGroups"))
	return w.InnerProvider.ListSecurityGroups()
}

func (w LoggedProvider) CreateSecurityGroup(
	name string, description string, rules []abstract.SecurityGroupRule,
) (*abstract.SecurityGroup, fail.Error) {
	defer w.prepare(w.trace("CreateSecurityGroup"))
	return w.InnerProvider.CreateSecurityGroup(name, description, rules)
}

func (w LoggedProvider) InspectSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	*abstract.SecurityGroup, fail.Error,
) {
	defer w.prepare(w.trace("InspectSecurityGroup"))
	return w.InnerProvider.InspectSecurityGroup(sgParam)
}

func (w LoggedProvider) ClearSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	*abstract.SecurityGroup, fail.Error,
) {
	defer w.prepare(w.trace("ClearSecurityGroup"))
	return w.InnerProvider.ClearSecurityGroup(sgParam)
}

func (w LoggedProvider) DeleteSecurityGroup(sgParam stacks.SecurityGroupParameter) fail.Error {
	defer w.prepare(w.trace("DeleteSecurityGroup"))
	return w.InnerProvider.DeleteSecurityGroup(sgParam)
}

func (w LoggedProvider) AddRuleToSecurityGroup(
	sgParam stacks.SecurityGroupParameter, rule abstract.SecurityGroupRule,
) (*abstract.SecurityGroup, fail.Error) {
	defer w.prepare(w.trace("AddRuleToSecurityGroup"))
	return w.InnerProvider.AddRuleToSecurityGroup(sgParam, rule)
}

func (w LoggedProvider) DeleteRuleFromSecurityGroup(
	sgParam stacks.SecurityGroupParameter, ruleID string,
) (*abstract.SecurityGroup, fail.Error) {
	defer w.prepare(w.trace("DeleteRuleFromSecurityGroup"))
	return w.InnerProvider.DeleteRuleFromSecurityGroup(sgParam, ruleID)
}

func (w LoggedProvider) WaitHostReady(hostParam stacks.HostParameter, timeout time.Duration) (
	*abstract.HostCore, fail.Error,
) {
	defer w.prepare(w.trace("WaitHostReady"))
	return w.InnerProvider.WaitHostReady(hostParam, timeout)
}

func (w LoggedProvider) BindSecurityGroupToHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) fail.Error {
	defer w.prepare(w.trace("BindSecurityGroupToHost"))
	return w.InnerProvider.BindSecurityGroupToHost(hostParam, sgParam)
}

func (w LoggedProvider) UnbindSecurityGroupFromHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) fail.Error {
	defer w.prepare(w.trace("UnbindSecurityGroupFromHost"))
	return w.InnerProvider.UnbindSecurityGroupFromHost(hostParam, sgParam)
}

func (w LoggedProvider) GetAuthenticationOptions() (Config, fail.Error) {
	defer w.prepare(w.trace("GetAuthenticationOptions"))
	return w.InnerProvider.GetAuthenticationOptions()
}

func (w LoggedProvider) GetConfigurationOptions() (Config, fail.Error) {
	defer w.prepare(w.trace("GetConfigurationOptions"))
	return w.InnerProvider.GetConfigurationOptions()
}

// Provider specific functions

// Build ...
func (w LoggedProvider) Build(something map[string]interface{}) (Provider, fail.Error) {
	defer w.prepare(w.trace("Build"))
	return w.InnerProvider.Build(something)
}

// ListImages ...
func (w LoggedProvider) ListImages(all bool) ([]abstract.Image, fail.Error) {
	defer w.prepare(w.trace("ListImages"))
	return w.InnerProvider.ListImages(all)
}

// ListTemplates ...
func (w LoggedProvider) ListTemplates(all bool) ([]abstract.HostTemplate, fail.Error) {
	defer w.prepare(w.trace("ListTemplates"))
	return w.InnerProvider.ListTemplates(all)
}

// InspectAuthenticationOptions ...
func (w LoggedProvider) InspectAuthenticationOptions() (Config, fail.Error) {
	defer w.prepare(w.trace("GetAuthenticationOptions"))
	return w.InnerProvider.GetAuthenticationOptions()
}

// InspectConfigurationOptions ...
func (w LoggedProvider) InspectConfigurationOptions() (Config, fail.Error) {
	defer w.prepare(w.trace("GetConfigurationOptions"))
	return w.InnerProvider.GetConfigurationOptions()
}

// InspectName ...
func (w LoggedProvider) GetName() string {
	defer w.prepare(w.trace("GetName"))
	return w.InnerProvider.GetName()
}

// InspectTenantParameters ...
func (w LoggedProvider) GetTenantParameters() map[string]interface{} {
	defer w.prepare(w.trace("GetTenantParameters"))
	return w.InnerProvider.GetTenantParameters()
}

// Stack specific functions

// trace ...
func (w LoggedProvider) trace(s string) (string, time.Time) {
	logrus.Tracef("stacks.%s::%s() called", w.Name, s)
	return s, time.Now()
}

// prepare ...
func (w LoggedProvider) prepare(s string, startTime time.Time) {
	logrus.Tracef("stacks.%s::%s() done in [%s]", w.Name, s, temporal.FormatDuration(time.Since(startTime)))
}

// NewLoggedProvider ...
func NewLoggedProvider(innerProvider Provider, name string) *LoggedProvider {
	lp := &LoggedProvider{InnerProvider: innerProvider, Name: name}

	var _ Provider = lp

	return lp
}

// ListAvailabilityZones ...
func (w LoggedProvider) ListAvailabilityZones() (map[string]bool, fail.Error) {
	defer w.prepare(w.trace("ListAvailabilityZones"))
	return w.InnerProvider.ListAvailabilityZones()
}

// ListRegions ...
func (w LoggedProvider) ListRegions() ([]string, fail.Error) {
	defer w.prepare(w.trace("ListRegions"))
	return w.InnerProvider.ListRegions()
}

// InspectImage ...
func (w LoggedProvider) InspectImage(id string) (*abstract.Image, fail.Error) {
	defer w.prepare(w.trace("GetImage"))
	return w.InnerProvider.InspectImage(id)
}

// InspectTemplate ...
func (w LoggedProvider) InspectTemplate(id string) (*abstract.HostTemplate, fail.Error) {
	defer w.prepare(w.trace("GetTemplate"))
	return w.InnerProvider.InspectTemplate(id)
}

// CreateKeyPair ...
func (w LoggedProvider) CreateKeyPair(name string) (*abstract.KeyPair, fail.Error) {
	defer w.prepare(w.trace("CreateKeyPair"))
	return w.InnerProvider.CreateKeyPair(name)
}

// InspectKeyPair ...
func (w LoggedProvider) InspectKeyPair(id string) (*abstract.KeyPair, fail.Error) {
	defer w.prepare(w.trace("GetKeyPair"))
	return w.InnerProvider.InspectKeyPair(id)
}

// ListKeyPairs ...
func (w LoggedProvider) ListKeyPairs() ([]abstract.KeyPair, fail.Error) {
	defer w.prepare(w.trace("ListKeyPairs"))
	return w.InnerProvider.ListKeyPairs()
}

// DeleteKeyPair ...
func (w LoggedProvider) DeleteKeyPair(id string) fail.Error {
	defer w.prepare(w.trace("DeleteKeyPair"))
	return w.InnerProvider.DeleteKeyPair(id)
}

// CreateNetwork ...
func (w LoggedProvider) CreateNetwork(req abstract.NetworkRequest) (*abstract.Network, fail.Error) {
	defer w.prepare(w.trace("CreateNetwork"))
	return w.InnerProvider.CreateNetwork(req)
}

// InspectNetwork ...
func (w LoggedProvider) InspectNetwork(id string) (*abstract.Network, fail.Error) {
	defer w.prepare(w.trace("GetNetwork"))
	return w.InnerProvider.InspectNetwork(id)
}

// InspectNetworkByName ...
func (w LoggedProvider) InspectNetworkByName(name string) (*abstract.Network, fail.Error) {
	defer w.prepare(w.trace("GetNetworkByName"))
	return w.InnerProvider.InspectNetworkByName(name)
}

// ListNetworks ...
func (w LoggedProvider) ListNetworks() ([]*abstract.Network, fail.Error) {
	defer w.prepare(w.trace("ListNetworks"))
	return w.InnerProvider.ListNetworks()
}

// DeleteNetwork ...
func (w LoggedProvider) DeleteNetwork(id string) fail.Error {
	defer w.prepare(w.trace("DeleteNetwork"))
	return w.InnerProvider.DeleteNetwork(id)
}

// CreateVIP ...
func (w LoggedProvider) CreateVIP(networkID string, description string) (*abstract.VirtualIP, fail.Error) {
	defer w.prepare(w.trace("CreateVIP"))
	return w.InnerProvider.CreateVIP(networkID, description)
}

// AddPublicIPToVIP adds a public IP to VIP
func (w LoggedProvider) AddPublicIPToVIP(vip *abstract.VirtualIP) fail.Error {
	defer w.prepare(w.trace("AddPublicIPToVIP"))
	return w.InnerProvider.AddPublicIPToVIP(vip)
}

// BindHostToVIP makes the host passed as parameter an allowed "target" of the VIP
func (w LoggedProvider) BindHostToVIP(vip *abstract.VirtualIP, hostID string) fail.Error {
	defer w.prepare(w.trace("BindHostToVIP"))
	return w.InnerProvider.BindHostToVIP(vip, hostID)
}

// UnbindHostFromVIP removes the bind between the VIP and a host
func (w LoggedProvider) UnbindHostFromVIP(vip *abstract.VirtualIP, hostID string) fail.Error {
	defer w.prepare(w.trace("UnbindHostFromVIP"))
	return w.InnerProvider.UnbindHostFromVIP(vip, hostID)
}

// DeleteVIP deletes the port corresponding to the VIP
func (w LoggedProvider) DeleteVIP(vip *abstract.VirtualIP) fail.Error {
	defer w.prepare(w.trace("DeleteVIP"))
	return w.InnerProvider.DeleteVIP(vip)
}

// CreateHost ...
func (w LoggedProvider) CreateHost(request abstract.HostRequest) (*abstract.HostFull, *userdata.Content, fail.Error) {
	defer w.prepare(w.trace("CreateHost"))
	return w.InnerProvider.CreateHost(request)
}

// InspectHost ...
func (w LoggedProvider) InspectHost(something stacks.HostParameter) (*abstract.HostFull, fail.Error) {
	defer w.prepare(w.trace("InspectHost"))
	return w.InnerProvider.InspectHost(something)
}

// InspectHostByName ...
func (w LoggedProvider) InspectHostByName(name string) (*abstract.HostCore, fail.Error) {
	defer w.prepare(w.trace("GetHostByName"))
	return w.InnerProvider.InspectHostByName(name)
}

// InspectHostState ...
func (w LoggedProvider) GetHostState(something stacks.HostParameter) (hoststate.Enum, fail.Error) {
	defer w.prepare(w.trace("GetHostState"))
	return w.InnerProvider.GetHostState(something)
}

// ListHosts ...
func (w LoggedProvider) ListHosts(b bool) (abstract.HostList, fail.Error) {
	defer w.prepare(w.trace("ListHosts"))
	return w.InnerProvider.ListHosts(b)
}

// DeleteHost ...
func (w LoggedProvider) DeleteHost(id stacks.HostParameter) fail.Error {
	defer w.prepare(w.trace("DeleteHost"))
	return w.InnerProvider.DeleteHost(id)
}

// StopHost ...
func (w LoggedProvider) StopHost(id stacks.HostParameter) fail.Error {
	defer w.prepare(w.trace("StopHost"))
	return w.InnerProvider.StopHost(id)
}

// StartHost ...
func (w LoggedProvider) StartHost(id stacks.HostParameter) fail.Error {
	defer w.prepare(w.trace("StartHost"))
	return w.InnerProvider.StartHost(id)
}

// RebootHost ...
func (w LoggedProvider) RebootHost(id stacks.HostParameter) fail.Error {
	defer w.prepare(w.trace("RebootHost"))
	return w.InnerProvider.RebootHost(id)
}

// ResizeHost ...
func (w LoggedProvider) ResizeHost(id stacks.HostParameter, request abstract.HostSizingRequirements) (*abstract.HostFull, fail.Error) {
	defer w.prepare(w.trace("ResizeHost"))
	return w.InnerProvider.ResizeHost(id, request)
}

// CreateVolume ...
func (w LoggedProvider) CreateVolume(request abstract.VolumeRequest) (*abstract.Volume, fail.Error) {
	defer w.prepare(w.trace("CreateVolume"))
	return w.InnerProvider.CreateVolume(request)
}

// InspectVolume ...
func (w LoggedProvider) InspectVolume(id string) (*abstract.Volume, fail.Error) {
	defer w.prepare(w.trace("GetVolume"))
	return w.InnerProvider.InspectVolume(id)
}

// ListVolumes ...
func (w LoggedProvider) ListVolumes() ([]abstract.Volume, fail.Error) {
	defer w.prepare(w.trace("ListVolumes"))
	return w.InnerProvider.ListVolumes()
}

// DeleteVolume ...
func (w LoggedProvider) DeleteVolume(id string) fail.Error {
	defer w.prepare(w.trace("DeleteVolume"))
	return w.InnerProvider.DeleteVolume(id)
}

// CreateVolumeAttachment ...
func (w LoggedProvider) CreateVolumeAttachment(request abstract.VolumeAttachmentRequest) (string, fail.Error) {
	defer w.prepare(w.trace("CreateVolumeAttachment"))
	return w.InnerProvider.CreateVolumeAttachment(request)
}

// InspectVolumeAttachment ...
func (w LoggedProvider) InspectVolumeAttachment(serverID, id string) (*abstract.VolumeAttachment, fail.Error) {
	defer w.prepare(w.trace("GetVolumeAttachment"))
	return w.InnerProvider.InspectVolumeAttachment(serverID, id)
}

// ListVolumeAttachments ...
func (w LoggedProvider) ListVolumeAttachments(serverID string) ([]abstract.VolumeAttachment, fail.Error) {
	defer w.prepare(w.trace("ListVolumeAttachments"))
	return w.InnerProvider.ListVolumeAttachments(serverID)
}

// DeleteVolumeAttachment ...
func (w LoggedProvider) DeleteVolumeAttachment(serverID, id string) fail.Error {
	defer w.prepare(w.trace("DeleteVolumeAttachment"))
	return w.InnerProvider.DeleteVolumeAttachment(serverID, id)
}

// InspectCapabilities returns the capabilities of the provider
func (w LoggedProvider) GetCapabilities() Capabilities {
	defer w.prepare(w.trace("Getcapabilities"))
	return w.InnerProvider.GetCapabilities()
}
