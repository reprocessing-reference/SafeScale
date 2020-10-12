/*
 * Copyright 2018-2020, CS Systemes d'Information, http://csgroup.eu
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package providers

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/lib/server/iaas/stacks"
	"github.com/CS-SI/SafeScale/lib/server/iaas/userdata"
	"github.com/CS-SI/SafeScale/lib/server/resources/abstract"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/hoststate"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
)

// ValidatedProvider ...
type ValidatedProvider WrappedProvider

func (w ValidatedProvider) InspectImage(id string) (*abstract.Image, fail.Error) {
	return w.InnerProvider.InspectImage(id)
}

func (w ValidatedProvider) InspectTemplate(id string) (*abstract.HostTemplate, fail.Error) {
	return w.InnerProvider.InspectTemplate(id)
}

func (w ValidatedProvider) InspectKeyPair(id string) (*abstract.KeyPair, fail.Error) {
	return w.InnerProvider.InspectKeyPair(id)
}

func (w ValidatedProvider) ListSecurityGroups() ([]*abstract.SecurityGroup, fail.Error) {
	return w.InnerProvider.ListSecurityGroups()
}

func (w ValidatedProvider) CreateSecurityGroup(
	name string, description string, rules []abstract.SecurityGroupRule,
) (*abstract.SecurityGroup, fail.Error) {
	return w.InnerProvider.CreateSecurityGroup(name, description, rules)
}

func (w ValidatedProvider) InspectSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	*abstract.SecurityGroup, fail.Error,
) {
	return w.InnerProvider.InspectSecurityGroup(sgParam)
}

func (w ValidatedProvider) ClearSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	*abstract.SecurityGroup, fail.Error,
) {
	return w.InnerProvider.ClearSecurityGroup(sgParam)
}

func (w ValidatedProvider) DeleteSecurityGroup(sgParam stacks.SecurityGroupParameter) fail.Error {
	return w.InnerProvider.DeleteSecurityGroup(sgParam)
}

func (w ValidatedProvider) AddRuleToSecurityGroup(
	sgParam stacks.SecurityGroupParameter, rule abstract.SecurityGroupRule,
) (*abstract.SecurityGroup, fail.Error) {
	return w.InnerProvider.AddRuleToSecurityGroup(sgParam, rule)
}

func (w ValidatedProvider) DeleteRuleFromSecurityGroup(
	sgParam stacks.SecurityGroupParameter, ruleID string,
) (*abstract.SecurityGroup, fail.Error) {
	return w.InnerProvider.DeleteRuleFromSecurityGroup(sgParam, ruleID)
}

func (w ValidatedProvider) InspectNetwork(id string) (*abstract.Network, fail.Error) {
	return w.InnerProvider.InspectNetwork(id)
}

func (w ValidatedProvider) InspectNetworkByName(name string) (*abstract.Network, fail.Error) {
	return w.InnerProvider.InspectNetworkByName(name)
}

func (w ValidatedProvider) InspectHostByName(s string) (*abstract.HostCore, fail.Error) {
	return w.InnerProvider.InspectHostByName(s)
}

func (w ValidatedProvider) BindSecurityGroupToHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) fail.Error {
	return w.InnerProvider.BindSecurityGroupToHost(hostParam, sgParam)
}

func (w ValidatedProvider) UnbindSecurityGroupFromHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) fail.Error {
	return w.InnerProvider.UnbindSecurityGroupFromHost(hostParam, sgParam)
}

func (w ValidatedProvider) InspectVolume(id string) (*abstract.Volume, fail.Error) {
	return w.InnerProvider.InspectVolume(id)
}

func (w ValidatedProvider) InspectVolumeAttachment(serverID, id string) (*abstract.VolumeAttachment, fail.Error) {
	return w.InnerProvider.InspectVolumeAttachment(serverID, id)
}

func (w ValidatedProvider) CreateVIP(netID string, name string) (*abstract.VirtualIP, fail.Error) {
	// FIXME: Add OK method to vip, then check return value
	vip, err := w.InnerProvider.CreateVIP(netID, name)
	return vip, err
}

func (w ValidatedProvider) AddPublicIPToVIP(vip *abstract.VirtualIP) fail.Error {
	// FIXME: Add OK method to vip
	return w.InnerProvider.AddPublicIPToVIP(vip)
}

func (w ValidatedProvider) BindHostToVIP(vip *abstract.VirtualIP, hostID string) fail.Error {
	// FIXME: Add OK method to vip
	return w.InnerProvider.BindHostToVIP(vip, hostID)
}

func (w ValidatedProvider) UnbindHostFromVIP(vip *abstract.VirtualIP, hostID string) fail.Error {
	// FIXME:  Add OK method to vip
	return w.InnerProvider.UnbindHostFromVIP(vip, hostID)
}

func (w ValidatedProvider) DeleteVIP(vip *abstract.VirtualIP) fail.Error {
	// FIXME: Add OK method to vip
	return w.InnerProvider.DeleteVIP(vip)
}

func (w ValidatedProvider) GetCapabilities() Capabilities {
	return w.InnerProvider.GetCapabilities()
}

func (w ValidatedProvider) GetTenantParameters() map[string]interface{} {
	return w.InnerProvider.GetTenantParameters()
}

// Provider specific functions

func (w ValidatedProvider) Build(something map[string]interface{}) (p Provider, xerr fail.Error) {
	return w.InnerProvider.Build(something)
}

func (w ValidatedProvider) ListImages(all bool) (res []abstract.Image, xerr fail.Error) {
	res, xerr = w.InnerProvider.ListImages(all)
	if xerr != nil {
		for _, image := range res {
			if !image.OK() {
				logrus.Warnf("Invalid image: %v", image)
			}
		}
	}
	return res, xerr
}

func (w ValidatedProvider) ListTemplates(all bool) (res []abstract.HostTemplate, xerr fail.Error) {
	res, xerr = w.InnerProvider.ListTemplates(all)
	if xerr != nil {
		for _, hostTemplate := range res {
			if !hostTemplate.OK() {
				logrus.Warnf("Invalid host template: %v", hostTemplate)
			}
		}
	}
	return res, xerr
}

func (w ValidatedProvider) GetAuthenticationOptions() (Config, fail.Error) {
	return w.InnerProvider.GetAuthenticationOptions()
}

func (w ValidatedProvider) GetConfigurationOptions() (Config, fail.Error) {
	return w.InnerProvider.GetConfigurationOptions()
}

func (w ValidatedProvider) GetName() string {
	return w.InnerProvider.GetName()
}

// Stack specific functions

// NewValidatedProvider ...
func NewValidatedProvider(innerProvider Provider, name string) *ValidatedProvider {
	vap := ValidatedProvider{InnerProvider: innerProvider, Name: name}

	var _ Provider = vap

	return &vap
}

// ListAvailabilityZones ...
func (w ValidatedProvider) ListAvailabilityZones() (map[string]bool, fail.Error) {
	return w.InnerProvider.ListAvailabilityZones()
}

// ListRegions ...
func (w ValidatedProvider) ListRegions() ([]string, fail.Error) {
	return w.InnerProvider.ListRegions()
}

// GetImage ...
func (w ValidatedProvider) GetImage(id string) (res *abstract.Image, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectImage(id)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid image: %v", *res)
			}
		}
	}

	return res, xerr
}

// GetTemplate ...
func (w ValidatedProvider) GetTemplate(id string) (res *abstract.HostTemplate, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectTemplate(id)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid template: %v", *res)
			}
		}
	}
	return res, xerr
}

// CreateKeyPair ...
func (w ValidatedProvider) CreateKeyPair(name string) (kp *abstract.KeyPair, xerr fail.Error) {
	kp, xerr = w.InnerProvider.CreateKeyPair(name)
	if xerr != nil {
		if kp == nil {
			logrus.Warn("Invalid keypair !")
		}
	}
	return kp, xerr
}

// GetKeyPair ...
func (w ValidatedProvider) GetKeyPair(id string) (kp *abstract.KeyPair, xerr fail.Error) {
	kp, xerr = w.InnerProvider.InspectKeyPair(id)
	if xerr != nil {
		if kp == nil {
			logrus.Warn("Invalid keypair !")
		}
	}
	return kp, xerr
}

// ListKeyPairs ...
func (w ValidatedProvider) ListKeyPairs() (res []abstract.KeyPair, xerr fail.Error) {
	return w.InnerProvider.ListKeyPairs()
}

// DeleteKeyPair ...
func (w ValidatedProvider) DeleteKeyPair(id string) (xerr fail.Error) {
	return w.InnerProvider.DeleteKeyPair(id)
}

// CreateNetwork ...
func (w ValidatedProvider) CreateNetwork(req abstract.NetworkRequest) (res *abstract.Network, xerr fail.Error) {
	res, xerr = w.InnerProvider.CreateNetwork(req)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid network: %v", *res)
			}
		}
	}
	return res, xerr
}

// GetNetwork ...
func (w ValidatedProvider) GetNetwork(id string) (res *abstract.Network, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectNetwork(id)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid network: %v", *res)
			}
		}
	}
	return res, xerr
}

// GetNetworkByName ...
func (w ValidatedProvider) GetNetworkByName(name string) (res *abstract.Network, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectNetworkByName(name)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid network: %v", *res)
			}
		}
	}
	return res, xerr
}

// ListNetworks ...
func (w ValidatedProvider) ListNetworks() (res []*abstract.Network, xerr fail.Error) {
	res, xerr = w.InnerProvider.ListNetworks()
	if xerr != nil {
		for _, item := range res {
			if item != nil {
				if !item.OK() {
					logrus.Warnf("Invalid network: %v", *item)
				}
			}
		}
	}
	return res, xerr
}

// DeleteNetwork ...
func (w ValidatedProvider) DeleteNetwork(id string) (xerr fail.Error) {
	return w.InnerProvider.DeleteNetwork(id)
}

// // CreateGateway ...
// func (w ValidatedProvider) CreateGateway(req abstract.GatewayRequest) (res *abstract.HostFull, data *userdata.Content, xerr fail.Error) {
// 	res, data, xerr = w.InnerProvider.CreateGateway(req)
// 	if xerr != nil {
// 		if res != nil {
// 			if !res.OK() {
// 				logrus.Warnf("Invalid host: %v", *res)
// 			}
// 		}
// 		if data != nil {
// 			if !data.OK() {
// 				logrus.Warnf("Invalid userdata: %v", *data)
// 			}
// 		}
// 	}
// 	return res, data, xerr
// }
//
// // DeleteGateway ...
// func (w ValidatedProvider) DeleteGateway(networkID string) (xerr fail.Error) {
// 	return w.InnerProvider.DeleteGateway(networkID)
// }

// CreateHost ...
func (w ValidatedProvider) CreateHost(request abstract.HostRequest) (res *abstract.HostFull, ud *userdata.Content, xerr fail.Error) {
	res, ud, xerr = w.InnerProvider.CreateHost(request)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid host: %v", *res)
			}
		}
		if ud != nil {
			if !ud.OK() {
				logrus.Warnf("Invalid userdata: %v", *ud)
			}
		}
	}
	return res, ud, xerr
}

// InspectHost ...
func (w ValidatedProvider) InspectHost(hostParam stacks.HostParameter) (res *abstract.HostFull, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectHost(hostParam)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid host: %v", *res)
			}
		}
	}
	return res, xerr
}

// WaitHostReady ...
func (w ValidatedProvider) WaitHostReady(hostParam stacks.HostParameter, timeout time.Duration) (*abstract.HostCore, fail.Error) {
	return w.InnerProvider.WaitHostReady(hostParam, timeout)
}

// GetHostByName ...
func (w ValidatedProvider) GetHostByName(name string) (res *abstract.HostCore, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectHostByName(name)
	if xerr != nil {
		if res != nil {
			logrus.Warnf("Invalid host: %v", *res)
		}
	}
	return res, xerr
}

// GetHostState ...
func (w ValidatedProvider) GetHostState(hostParam stacks.HostParameter) (res hoststate.Enum, xerr fail.Error) {
	return w.InnerProvider.GetHostState(hostParam)
}

// ListHosts ...
func (w ValidatedProvider) ListHosts(details bool) (res abstract.HostList, xerr fail.Error) {
	res, xerr = w.InnerProvider.ListHosts(details)
	if xerr != nil {
		for _, item := range res {
			if item != nil {
				if !item.OK() {
					logrus.Warnf("Invalid host: %v", *item)
				}
			}
		}
	}
	return res, xerr
}

// DeleteHost ...
func (w ValidatedProvider) DeleteHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	return w.InnerProvider.DeleteHost(hostParam)
}

// StopHost ...
func (w ValidatedProvider) StopHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	return w.InnerProvider.StopHost(hostParam)
}

// StartHost ...
func (w ValidatedProvider) StartHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	return w.InnerProvider.StartHost(hostParam)
}

// RebootHost ...
func (w ValidatedProvider) RebootHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	return w.InnerProvider.RebootHost(hostParam)
}

// ResizeHost ...
func (w ValidatedProvider) ResizeHost(hostParam stacks.HostParameter, request abstract.HostSizingRequirements) (res *abstract.HostFull, xerr fail.Error) {
	res, xerr = w.InnerProvider.ResizeHost(hostParam, request)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid host: %v", *res)
			}
		}
	}
	return res, xerr
}

// CreateVolume ...
func (w ValidatedProvider) CreateVolume(request abstract.VolumeRequest) (res *abstract.Volume, xerr fail.Error) {
	res, xerr = w.InnerProvider.CreateVolume(request)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid volume: %v", *res)
			}
		}
	}
	return res, xerr
}

// GetVolume ...
func (w ValidatedProvider) GetVolume(id string) (res *abstract.Volume, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectVolume(id)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid volume: %v", *res)
			}
		}
	}
	return res, xerr
}

// ListVolumes ...
func (w ValidatedProvider) ListVolumes() (res []abstract.Volume, xerr fail.Error) {
	res, xerr = w.InnerProvider.ListVolumes()
	if xerr != nil {
		for _, item := range res {
			if !item.OK() {
				logrus.Warnf("Invalid host: %v", item)
			}
		}
	}
	return res, xerr
}

// DeleteVolume ...
func (w ValidatedProvider) DeleteVolume(id string) (xerr fail.Error) {
	return w.InnerProvider.DeleteVolume(id)
}

// CreateVolumeAttachment ...
func (w ValidatedProvider) CreateVolumeAttachment(request abstract.VolumeAttachmentRequest) (id string, xerr fail.Error) {
	return w.InnerProvider.CreateVolumeAttachment(request)
}

// GetVolumeAttachment ...
func (w ValidatedProvider) GetVolumeAttachment(serverID, id string) (res *abstract.VolumeAttachment, xerr fail.Error) {
	res, xerr = w.InnerProvider.InspectVolumeAttachment(serverID, id)
	if xerr != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid volume attachment: %v", *res)
			}
		}
	}
	return res, xerr
}

// ListVolumeAttachments ...
func (w ValidatedProvider) ListVolumeAttachments(serverID string) (res []abstract.VolumeAttachment, xerr fail.Error) {
	res, xerr = w.InnerProvider.ListVolumeAttachments(serverID)
	if xerr != nil {
		for _, item := range res {
			if !item.OK() {
				logrus.Warnf("Invalid volume attachment: %v", item)
			}
		}
	}
	return res, xerr
}

// DeleteVolumeAttachment ...
func (w ValidatedProvider) DeleteVolumeAttachment(serverID, id string) (xerr fail.Error) {
	return w.InnerProvider.DeleteVolumeAttachment(serverID, id)
}
