/*
 * Copyright 2018-2020, CS Systemes d'Information, http://www.c-s.fr
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

func (w ValidatedProvider) ListSecurityGroups() (_ []*abstract.SecurityGroup, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.ListSecurityGroups()
}

func (w ValidatedProvider) CreateSecurityGroup(
	name string, description string, rules []abstract.SecurityGroupRule,
) (_ *abstract.SecurityGroup, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.CreateSecurityGroup(name, description, rules)
}

func (w ValidatedProvider) InspectSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	_ *abstract.SecurityGroup, err fail.Error,
) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.InspectSecurityGroup(sgParam)
}

func (w ValidatedProvider) ClearSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	_ *abstract.SecurityGroup, err fail.Error,
) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.ClearSecurityGroup(sgParam)
}

func (w ValidatedProvider) DeleteSecurityGroup(sgParam stacks.SecurityGroupParameter) (err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.DeleteSecurityGroup(sgParam)
}

func (w ValidatedProvider) AddRuleToSecurityGroup(
	sgParam stacks.SecurityGroupParameter, rule abstract.SecurityGroupRule,
) (_ *abstract.SecurityGroup, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.AddRuleToSecurityGroup(sgParam, rule)
}

func (w ValidatedProvider) DeleteRuleFromSecurityGroup(
	sgParam stacks.SecurityGroupParameter, ruleID string,
) (_ *abstract.SecurityGroup, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.DeleteRuleFromSecurityGroup(sgParam, ruleID)
}

func (w ValidatedProvider) WaitHostReady(hostParam stacks.HostParameter, timeout time.Duration) (
	_ *abstract.HostCore, err fail.Error,
) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.WaitHostReady(hostParam, timeout)
}

func (w ValidatedProvider) BindSecurityGroupToHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) (err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.BindSecurityGroupToHost(hostParam, sgParam)
}

func (w ValidatedProvider) UnbindSecurityGroupFromHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) (err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.UnbindSecurityGroupFromHost(hostParam, sgParam)
}

func (w ValidatedProvider) CreateVIP(first string, second string) (_ *abstract.VirtualIP, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.CreateVIP(first, second)
}

func (w ValidatedProvider) AddPublicIPToVIP(res *abstract.VirtualIP) (err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.AddPublicIPToVIP(res)
}

func (w ValidatedProvider) BindHostToVIP(vip *abstract.VirtualIP, hostID string) (err fail.Error) {
	defer fail.OnPanic(&err)

	if vip == nil {
		return fail.InvalidParameterError("vip", "cannot be nil")
	}
	if hostID == "" {
		return fail.InvalidParameterError("host", "cannot be empty string")
	}

	return w.InnerProvider.BindHostToVIP(vip, hostID)
}

func (w ValidatedProvider) UnbindHostFromVIP(vip *abstract.VirtualIP, hostID string) (err fail.Error) {
	defer fail.OnPanic(&err)

	if vip == nil {
		return fail.InvalidParameterError("vip", "cannot be nil")
	}
	if hostID == "" {
		return fail.InvalidParameterError("host", "cannot be empty string")
	}

	return w.InnerProvider.UnbindHostFromVIP(vip, hostID)
}

func (w ValidatedProvider) DeleteVIP(vip *abstract.VirtualIP) (err fail.Error) {
	defer fail.OnPanic(&err)

	if vip == nil {
		return fail.InvalidParameterError("vip", "cannot be nil")
	}

	return w.InnerProvider.DeleteVIP(vip)
}

func (w ValidatedProvider) GetCapabilities() Capabilities {
	return w.InnerProvider.GetCapabilities()
}

func (w ValidatedProvider) GetTenantParameters() map[string]interface{} {
	return w.InnerProvider.GetTenantParameters()
}

// Provider specific functions

func (w ValidatedProvider) Build(something map[string]interface{}) (p Provider, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.Build(something)
}

func (w ValidatedProvider) ListImages(all bool) (res []abstract.Image, err fail.Error) {
	defer fail.OnPanic(&err)

	res, err = w.InnerProvider.ListImages(all)
	if err != nil {
		for _, image := range res {
			if !image.OK() {
				logrus.Warnf("Invalid image: %v", image)
			}
		}
	}
	return res, err
}

func (w ValidatedProvider) ListTemplates(all bool) (res []abstract.HostTemplate, err fail.Error) {
	defer fail.OnPanic(&err)

	res, err = w.InnerProvider.ListTemplates(all)
	if err != nil {
		for _, hostTemplate := range res {
			if !hostTemplate.OK() {
				logrus.Warnf("Invalid host template: %v", hostTemplate)
			}
		}
	}
	return res, err
}

func (w ValidatedProvider) GetAuthenticationOptions() (_ Config, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.GetAuthenticationOptions()
}

func (w ValidatedProvider) GetConfigurationOptions() (_ Config, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.GetConfigurationOptions()
}

func (w ValidatedProvider) GetName() string {
	return w.InnerProvider.GetName()
}

// Stack specific functions

// NewValidatedProvider ...
func NewValidatedProvider(InnerProvider Provider, name string) *ValidatedProvider {

	// Feel the pain
	w := &ValidatedProvider{InnerProvider: InnerProvider, Name: name}

	// make sure there are no missing unimplemented methods
	var _ Provider = w

	return w
}

// ListAvailabilityZones ...
func (w ValidatedProvider) ListAvailabilityZones() (_ map[string]bool, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.ListAvailabilityZones()
}

// ListRegions ...
func (w ValidatedProvider) ListRegions() (_ []string, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.ListRegions()
}

// InspectImage ...
func (w ValidatedProvider) InspectImage(id string) (res *abstract.Image, err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return nil, fail.InvalidParameterError("id", "cannot be empty string")
	}

	res, err = w.InnerProvider.InspectImage(id)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid image: %v", *res)
			}
		}
	}

	return res, err
}

// InspectTemplate ...
func (w ValidatedProvider) InspectTemplate(id string) (res *abstract.HostTemplate, err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return nil, fail.InvalidParameterError("id", "cannot be empty string")
	}

	res, err = w.InnerProvider.InspectTemplate(id)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid template: %v", *res)
			}
		}
	}
	return res, err
}

// CreateKeyPair ...
func (w ValidatedProvider) CreateKeyPair(name string) (kp *abstract.KeyPair, err fail.Error) {
	defer fail.OnPanic(&err)

	if name == "" {
		return nil, fail.InvalidParameterError("name", "cannot be empty string")
	}

	kp, err = w.InnerProvider.CreateKeyPair(name)
	if err != nil {
		if kp == nil {
			logrus.Warn("Invalid keypair !")
		}
	}
	return kp, err
}

// InspectKeyPair ...
func (w ValidatedProvider) InspectKeyPair(id string) (kp *abstract.KeyPair, err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return nil, fail.InvalidParameterError("id", "cannot be nil")
	}

	kp, err = w.InnerProvider.InspectKeyPair(id)
	if err != nil {
		if kp == nil {
			logrus.Warn("Invalid keypair !")
		}
	}
	return kp, err
}

// ListKeyPairs ...
func (w ValidatedProvider) ListKeyPairs() (res []abstract.KeyPair, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.ListKeyPairs()
}

// DeleteKeyPair ...
func (w ValidatedProvider) DeleteKeyPair(id string) (err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return fail.InvalidParameterError("id", "cannot be empty string")
	}

	return w.InnerProvider.DeleteKeyPair(id)
}

// CreateNetwork ...
func (w ValidatedProvider) CreateNetwork(req abstract.NetworkRequest) (res *abstract.Network, err fail.Error) {
	defer fail.OnPanic(&err)

	res, err = w.InnerProvider.CreateNetwork(req)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid network: %v", *res)
			}
		}
	}
	return res, err
}

// InspectNetwork ...
func (w ValidatedProvider) InspectNetwork(id string) (res *abstract.Network, err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return nil, fail.InvalidParameterError("id", "cannot be empty string")
	}

	res, err = w.InnerProvider.InspectNetwork(id)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid network: %v", *res)
			}
		}
	}
	return res, err
}

// InspectNetworkByName ...
func (w ValidatedProvider) InspectNetworkByName(name string) (res *abstract.Network, err fail.Error) {
	defer fail.OnPanic(&err)

	if name == "" {
		return nil, fail.InvalidParameterError("name", "cannot be empty string")
	}

	res, err = w.InnerProvider.InspectNetworkByName(name)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid network: %v", *res)
			}
		}
	}
	return res, err
}

// ListNetworks ...
func (w ValidatedProvider) ListNetworks() (res []*abstract.Network, err fail.Error) {
	defer fail.OnPanic(&err)

	res, err = w.InnerProvider.ListNetworks()
	if err != nil {
		for _, item := range res {
			if item != nil {
				if !item.OK() {
					logrus.Warnf("Invalid network: %v", *item)
				}
			}
		}
	}
	return res, err
}

// DeleteNetwork ...
func (w ValidatedProvider) DeleteNetwork(id string) (err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.DeleteNetwork(id)
}

// CreateHost ...
func (w ValidatedProvider) CreateHost(request abstract.HostRequest) (res *abstract.HostFull, data *userdata.Content, err fail.Error) {
	defer fail.OnPanic(&err)

	if request.KeyPair == nil {
		return nil, nil, fail.InvalidParameterError("request.KeyPair", "cannot be nil")
	}

	res, data, err = w.InnerProvider.CreateHost(request)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid host: %v", *res)
			}
		}
		if data != nil {
			if !data.OK() {
				logrus.Warnf("Invalid userdata: %v", *data)
			}
		}
	}
	return res, data, err
}

// InspectHost ...
func (w ValidatedProvider) InspectHost(something stacks.HostParameter) (res *abstract.HostFull, err fail.Error) {
	defer fail.OnPanic(&err)

	res, err = w.InnerProvider.InspectHost(something)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid host: %v", *res)
			}
		}
	}
	return res, err
}

// InspectHostByName ...
func (w ValidatedProvider) InspectHostByName(name string) (res *abstract.HostCore, err fail.Error) {
	defer fail.OnPanic(&err)

	if name == "" {
		return nil, fail.InvalidParameterError("name", "cannot be empty string")
	}

	res, err = w.InnerProvider.InspectHostByName(name)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid host: %v", *res)
			}
		}
	}
	return res, err
}

// InspectHostState ...
func (w ValidatedProvider) GetHostState(something stacks.HostParameter) (res hoststate.Enum, err fail.Error) {
	defer fail.OnPanic(&err)

	return w.InnerProvider.GetHostState(something)
}

// ListHosts ...
func (w ValidatedProvider) ListHosts(b bool) (res abstract.HostList, err fail.Error) {
	defer fail.OnPanic(&err)

	res, err = w.InnerProvider.ListHosts(b)
	if err != nil {
		for _, item := range res {
			if item != nil {
				if !item.OK() {
					logrus.Warnf("Invalid host: %v", *item)
				}
			}
		}
	}
	return res, err
}

// DeleteHost ...
func (w ValidatedProvider) DeleteHost(id stacks.HostParameter) (err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return fail.InvalidParameterError("id", "cannot be empty string")
	}

	return w.InnerProvider.DeleteHost(id)
}

// StopHost ...
func (w ValidatedProvider) StopHost(id stacks.HostParameter) (err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return fail.InvalidParameterError("id", "cannot be empty string")
	}

	return w.InnerProvider.StopHost(id)
}

// StartHost ...
func (w ValidatedProvider) StartHost(id stacks.HostParameter) (err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return fail.InvalidParameterError("id", "cannot be empty string")
	}

	return w.InnerProvider.StartHost(id)
}

// RebootHost ...
func (w ValidatedProvider) RebootHost(id stacks.HostParameter) (err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return fail.InvalidParameterError("id", "cannot be empty string")
	}

	return w.InnerProvider.RebootHost(id)
}

// ResizeHost ...
func (w ValidatedProvider) ResizeHost(id stacks.HostParameter, request abstract.HostSizingRequirements) (res *abstract.HostFull, err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return nil, fail.InvalidParameterError("id", "cannot be empty string")
	}

	res, err = w.InnerProvider.ResizeHost(id, request)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid host: %v", *res)
			}
		}
	}
	return res, err
}

// CreateVolume ...
func (w ValidatedProvider) CreateVolume(request abstract.VolumeRequest) (res *abstract.Volume, err fail.Error) {
	defer fail.OnPanic(&err)

	if request.Name == "" {
		return nil, fail.InvalidParameterError("request.Name", "cannot be empty string")
	}

	res, err = w.InnerProvider.CreateVolume(request)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid volume: %v", *res)
			}
		}
	}
	return res, err
}

// InspectVolume ...
func (w ValidatedProvider) InspectVolume(id string) (res *abstract.Volume, err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return nil, fail.InvalidParameterError("id", "cannot be empty string")
	}

	res, err = w.InnerProvider.InspectVolume(id)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid volume: %v", *res)
			}
		}
	}
	return res, err
}

// ListVolumes ...
func (w ValidatedProvider) ListVolumes() (res []abstract.Volume, err fail.Error) {
	defer fail.OnPanic(&err)

	res, err = w.InnerProvider.ListVolumes()
	if err != nil {
		for _, item := range res {
			if !item.OK() {
				logrus.Warnf("Invalid host: %v", item)
			}
		}
	}
	return res, err
}

// DeleteVolume ...
func (w ValidatedProvider) DeleteVolume(id string) (err fail.Error) {
	defer fail.OnPanic(&err)

	if id == "" {
		return fail.InvalidParameterError("id", "cannot be empty string")
	}

	return w.InnerProvider.DeleteVolume(id)
}

// CreateVolumeAttachment ...
func (w ValidatedProvider) CreateVolumeAttachment(request abstract.VolumeAttachmentRequest) (parameter string, err fail.Error) {
	defer fail.OnPanic(&err)

	if request.Name == "" {
		return "", fail.InvalidParameterError("request.Name", "cannot be empty string")
	}
	if request.HostID == "" {
		return "", fail.InvalidParameterError("HostID", "cannot be empty string")
	}
	if request.VolumeID == "" {
		return "", fail.InvalidParameterError("VolumeID", "cannot be empty string")
	}

	return w.InnerProvider.CreateVolumeAttachment(request)
}

// InspectVolumeAttachment ...
func (w ValidatedProvider) InspectVolumeAttachment(serverID, id string) (res *abstract.VolumeAttachment, err fail.Error) {
	defer fail.OnPanic(&err)

	if serverID == "" {
		return nil, fail.InvalidParameterError("serverID", "cannot be empty string")
	}
	if id == "" {
		return nil, fail.InvalidParameterError("id", "cannot be empty string")
	}

	res, err = w.InnerProvider.InspectVolumeAttachment(serverID, id)
	if err != nil {
		if res != nil {
			if !res.OK() {
				logrus.Warnf("Invalid volume attachment: %v", *res)
			}
		}
	}
	return res, err
}

// ListVolumeAttachments ...
func (w ValidatedProvider) ListVolumeAttachments(serverID string) (res []abstract.VolumeAttachment, err fail.Error) {
	defer fail.OnPanic(&err)

	if serverID == "" {
		return nil, fail.InvalidParameterError("serverID", "cannot be empty string")
	}

	res, err = w.InnerProvider.ListVolumeAttachments(serverID)
	if err != nil {
		for _, item := range res {
			if !item.OK() {
				logrus.Warnf("Invalid volume attachment: %v", item)
			}
		}
	}
	return res, err
}

// DeleteVolumeAttachment ...
func (w ValidatedProvider) DeleteVolumeAttachment(serverID, vaID string) (err fail.Error) {
	defer fail.OnPanic(&err)

	if serverID == "" {
		return fail.InvalidParameterError("serverID", "cannot be empty string")
	}
	if vaID == "" {
		return fail.InvalidParameterError("vaID", "cannot be empty string")
	}

	return w.InnerProvider.DeleteVolumeAttachment(serverID, vaID)
}
