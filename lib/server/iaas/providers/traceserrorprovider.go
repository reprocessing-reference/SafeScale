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
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/lib/server/iaas/stacks"
	"github.com/CS-SI/SafeScale/lib/server/iaas/userdata"
	"github.com/CS-SI/SafeScale/lib/server/resources/abstract"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/hoststate"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
)

// ErrorTraceProvider ...
type ErrorTraceProvider WrappedProvider

// WaitHostReady ...
func (w ErrorTraceProvider) WaitHostReady(hostParam stacks.HostParameter, timeout time.Duration) (_ *abstract.HostCore, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:WaitHostReady", w.Name))
	return w.InnerProvider.WaitHostReady(hostParam, timeout)
}

// Provider specific functions

// Build ...
func (w ErrorTraceProvider) Build(something map[string]interface{}) (p Provider, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:Build", w.Name))
	return w.InnerProvider.Build(something)
}

// ListImages ...
func (w ErrorTraceProvider) ListImages(all bool) (images []abstract.Image, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListImages", w.Name))
	return w.InnerProvider.ListImages(all)
}

// ListTemplates ...
func (w ErrorTraceProvider) ListTemplates(all bool) (templates []abstract.HostTemplate, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListTemplates", w.Name))
	return w.InnerProvider.ListTemplates(all)
}

// GetAuthenticationOptions ...
func (w ErrorTraceProvider) GetAuthenticationOptions() (cfg Config, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:GetAuthenticationOptions", w.Name))

	return w.InnerProvider.GetAuthenticationOptions()
}

// GetConfigurationOptions ...
func (w ErrorTraceProvider) GetConfigurationOptions() (cfg Config, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:GetConfigurationOptions", w.Name))
	return w.InnerProvider.GetConfigurationOptions()
}

// GetName ...
func (w ErrorTraceProvider) GetName() string {
	return w.InnerProvider.GetName()
}

// GetTenantParameters ...
func (w ErrorTraceProvider) GetTenantParameters() map[string]interface{} {
	return w.InnerProvider.GetTenantParameters()
}

// Stack specific functions

// NewErrorTraceProvider ...
func NewErrorTraceProvider(innerProvider Provider, name string) ErrorTraceProvider {
	return ErrorTraceProvider{InnerProvider: innerProvider, Name: name}
}

// ListAvailabilityZones ...
func (w ErrorTraceProvider) ListAvailabilityZones() (zones map[string]bool, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListAvailabilityZones", w.Name))
	return w.InnerProvider.ListAvailabilityZones()
}

// ListRegions ...
func (w ErrorTraceProvider) ListRegions() (regions []string, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListRegions", w.Name))
	return w.InnerProvider.ListRegions()
}

// GetImage ...
func (w ErrorTraceProvider) GetImage(id string) (images *abstract.Image, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectImage", w.Name))
	return w.InnerProvider.InspectImage(id)
}

// GetTemplate ...
func (w ErrorTraceProvider) GetTemplate(id string) (templates *abstract.HostTemplate, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectTemplate", w.Name))
	return w.InnerProvider.InspectTemplate(id)
}

// CreateKeyPair ...
func (w ErrorTraceProvider) CreateKeyPair(name string) (pairs *abstract.KeyPair, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:CreateKeyPair", w.Name))
	return w.InnerProvider.CreateKeyPair(name)
}

// GetKeyPair ...
func (w ErrorTraceProvider) GetKeyPair(id string) (pairs *abstract.KeyPair, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectKeyPair", w.Name))
	return w.InnerProvider.InspectKeyPair(id)
}

// ListKeyPairs ...
func (w ErrorTraceProvider) ListKeyPairs() (pairs []abstract.KeyPair, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListKeyPairs", w.Name))
	return w.InnerProvider.ListKeyPairs()
}

// DeleteKeyPair ...
func (w ErrorTraceProvider) DeleteKeyPair(id string) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:DeleteKeyPair", w.Name))
	return w.InnerProvider.DeleteKeyPair(id)
}

// CreateNetwork ...
func (w ErrorTraceProvider) CreateNetwork(req abstract.NetworkRequest) (net *abstract.Network, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:CreateNetwork", w.Name))
	return w.InnerProvider.CreateNetwork(req)
}

// GetNetwork ...
func (w ErrorTraceProvider) GetNetwork(id string) (net *abstract.Network, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectNetwork", w.Name))
	return w.InnerProvider.InspectNetwork(id)
}

// GetNetworkByName ...
func (w ErrorTraceProvider) GetNetworkByName(name string) (net *abstract.Network, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectNetworkByName", w.Name))
	return w.InnerProvider.InspectNetworkByName(name)
}

// ListNetworks ...
func (w ErrorTraceProvider) ListNetworks() (net []*abstract.Network, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListNetworks", w.Name))
	return w.InnerProvider.ListNetworks()
}

// DeleteNetwork ...
func (w ErrorTraceProvider) DeleteNetwork(id string) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:DeleteNetwork", w.Name))
	return w.InnerProvider.DeleteNetwork(id)
}

// // CreateGateway ...
// func (w ErrorTraceProvider) CreateGateway(req abstract.GatewayRequest) (_ *abstract.HostFull, _ *userdata.Content, xerr fail.Error) {
// 	defer func(prefix string) {
// 		if xerr != nil {
// 			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
// 		}
// 	}(fmt.Sprintf("%s:CreateGateway", w.Name))
// 	return w.InnerProvider.CreateGateway(req)
// }
//
// // DeleteGateway ...
// func (w ErrorTraceProvider) DeleteGateway(networkID string) (xerr fail.Error) {
// 	defer func(prefix string) {
// 		if xerr != nil {
// 			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
// 		}
// 	}(fmt.Sprintf("%s:DeleteGateway", w.Name))
// 	return w.InnerProvider.DeleteGateway(networkID)
// }

// CreateVIP ...
func (w ErrorTraceProvider) CreateVIP(networkID string, description string) (_ *abstract.VirtualIP, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:CreateVIP", w.Name))
	return w.InnerProvider.CreateVIP(networkID, description)
}

// AddPublicIPToVIP adds a public IP to VIP
func (w ErrorTraceProvider) AddPublicIPToVIP(vip *abstract.VirtualIP) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:AddPublicIPToVIP", w.Name))
	return w.InnerProvider.AddPublicIPToVIP(vip)
}

// BindHostToVIP makes the host passed as parameter an allowed "target" of the VIP
func (w ErrorTraceProvider) BindHostToVIP(vip *abstract.VirtualIP, hostID string) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:BindHostToVIP", w.Name))
	return w.InnerProvider.BindHostToVIP(vip, hostID)
}

// UnbindHostFromVIP removes the bind between the VIP and a host
func (w ErrorTraceProvider) UnbindHostFromVIP(vip *abstract.VirtualIP, hostID string) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:UnbindHostFromVIP", w.Name))
	return w.InnerProvider.UnbindHostFromVIP(vip, hostID)
}

// DeleteVIP deletes the port corresponding to the VIP
func (w ErrorTraceProvider) DeleteVIP(vip *abstract.VirtualIP) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:DeleteVIP", w.Name))
	return w.InnerProvider.DeleteVIP(vip)
}

// CreateHost ...
func (w ErrorTraceProvider) CreateHost(request abstract.HostRequest) (_ *abstract.HostFull, _ *userdata.Content, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:CreateHost", w.Name))
	return w.InnerProvider.CreateHost(request)
}

// InspectHost ...
func (w ErrorTraceProvider) InspectHost(hostParam stacks.HostParameter) (_ *abstract.HostFull, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectHost", w.Name))
	return w.InnerProvider.InspectHost(hostParam)
}

// GetHostByName ...
func (w ErrorTraceProvider) GetHostByName(name string) (_ *abstract.HostCore, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectHostByName", w.Name))
	return w.InnerProvider.InspectHostByName(name)
}

// GetHostState ...
func (w ErrorTraceProvider) GetHostState(hostParam stacks.HostParameter) (_ hoststate.Enum, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:GetHostState", w.Name))
	return w.InnerProvider.GetHostState(hostParam)
}

// ListHosts ...
func (w ErrorTraceProvider) ListHosts(details bool) (_ abstract.HostList, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListHosts", w.Name))
	return w.InnerProvider.ListHosts(details)
}

// DeleteHost ...
func (w ErrorTraceProvider) DeleteHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:DeleteHost", w.Name))
	return w.InnerProvider.DeleteHost(hostParam)
}

// StopHost ...
func (w ErrorTraceProvider) StopHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:StopHost", w.Name))
	return w.InnerProvider.StopHost(hostParam)
}

// StartHost ...
func (w ErrorTraceProvider) StartHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:StartHost", w.Name))
	return w.InnerProvider.StartHost(hostParam)
}

// RebootHost ...
func (w ErrorTraceProvider) RebootHost(hostParam stacks.HostParameter) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:RebootHost", w.Name))
	return w.InnerProvider.RebootHost(hostParam)
}

// ResizeHost ...
func (w ErrorTraceProvider) ResizeHost(hostParam stacks.HostParameter, request abstract.HostSizingRequirements) (_ *abstract.HostFull, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ResizeHost", w.Name))
	return w.InnerProvider.ResizeHost(hostParam, request)
}

// CreateVolume ...
func (w ErrorTraceProvider) CreateVolume(request abstract.VolumeRequest) (_ *abstract.Volume, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:CreateVolume", w.Name))
	return w.InnerProvider.CreateVolume(request)
}

// GetVolume ...
func (w ErrorTraceProvider) GetVolume(id string) (_ *abstract.Volume, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectVolume", w.Name))
	return w.InnerProvider.InspectVolume(id)
}

// ListVolumes ...
func (w ErrorTraceProvider) ListVolumes() (_ []abstract.Volume, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListVolumes", w.Name))
	return w.InnerProvider.ListVolumes()
}

// DeleteVolume ...
func (w ErrorTraceProvider) DeleteVolume(id string) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:DeleteVolume", w.Name))
	return w.InnerProvider.DeleteVolume(id)
}

// CreateVolumeAttachment ...
func (w ErrorTraceProvider) CreateVolumeAttachment(request abstract.VolumeAttachmentRequest) (_ string, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:CreateVolumeAttachment", w.Name))
	return w.InnerProvider.CreateVolumeAttachment(request)
}

// GetVolumeAttachment ...
func (w ErrorTraceProvider) GetVolumeAttachment(serverID, id string) (_ *abstract.VolumeAttachment, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:InspectVolumeAttachment", w.Name))
	return w.InnerProvider.InspectVolumeAttachment(serverID, id)
}

// ListVolumeAttachments ...
func (w ErrorTraceProvider) ListVolumeAttachments(serverID string) (_ []abstract.VolumeAttachment, xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:ListVolumeAttachments", w.Name))
	return w.InnerProvider.ListVolumeAttachments(serverID)
}

// DeleteVolumeAttachment ...
func (w ErrorTraceProvider) DeleteVolumeAttachment(serverID, id string) (xerr fail.Error) {
	defer func(prefix string) {
		if xerr != nil {
			logrus.Warnf("%s : Intercepted error: %v", prefix, xerr)
		}
	}(fmt.Sprintf("%s:DeleteVolumeAttachment", w.Name))
	return w.InnerProvider.DeleteVolumeAttachment(serverID, id)
}

// GetCapabilities ...
func (w ErrorTraceProvider) GetCapabilities() Capabilities {
	return w.InnerProvider.GetCapabilities()
}
