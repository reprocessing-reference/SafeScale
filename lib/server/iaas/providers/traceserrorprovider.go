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

func (w ErrorTraceProvider) ListSecurityGroups() (_ []*abstract.SecurityGroup, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListSecurityGroups", w.Name))
	return w.InnerProvider.ListSecurityGroups()
}

func (w ErrorTraceProvider) CreateSecurityGroup(
	name string, description string, rules []abstract.SecurityGroupRule,
) (_ *abstract.SecurityGroup, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:CreateSecurityGroup", w.Name))
	return w.InnerProvider.CreateSecurityGroup(name, description, rules)
}

func (w ErrorTraceProvider) InspectSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	_ *abstract.SecurityGroup, err fail.Error,
) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:InspectSecurityGroup", w.Name))
	return w.InnerProvider.InspectSecurityGroup(sgParam)
}

func (w ErrorTraceProvider) ClearSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	_ *abstract.SecurityGroup, err fail.Error,
) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ClearSecurityGroup", w.Name))
	return w.InnerProvider.ClearSecurityGroup(sgParam)
}

func (w ErrorTraceProvider) DeleteSecurityGroup(sgParam stacks.SecurityGroupParameter) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteSecurityGroup", w.Name))
	return w.InnerProvider.DeleteSecurityGroup(sgParam)
}

func (w ErrorTraceProvider) AddRuleToSecurityGroup(
	sgParam stacks.SecurityGroupParameter, rule abstract.SecurityGroupRule,
) (_ *abstract.SecurityGroup, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:AddRuleToSecurityGroup", w.Name))
	return w.InnerProvider.AddRuleToSecurityGroup(sgParam, rule)
}

func (w ErrorTraceProvider) DeleteRuleFromSecurityGroup(
	sgParam stacks.SecurityGroupParameter, ruleID string,
) (_ *abstract.SecurityGroup, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteRuleFromSecurityGroup", w.Name))
	return w.InnerProvider.DeleteRuleFromSecurityGroup(sgParam, ruleID)
}

func (w ErrorTraceProvider) WaitHostReady(hostParam stacks.HostParameter, timeout time.Duration) (
	_ *abstract.HostCore, err fail.Error,
) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:WaitHostReady", w.Name))
	return w.InnerProvider.WaitHostReady(hostParam, timeout)
}

func (w ErrorTraceProvider) BindSecurityGroupToHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:BindSecurityGroupToHost", w.Name))
	return w.InnerProvider.BindSecurityGroupToHost(hostParam, sgParam)
}

func (w ErrorTraceProvider) UnbindSecurityGroupFromHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:UnbindSecurityGroupFromHost", w.Name))
	return w.InnerProvider.UnbindSecurityGroupFromHost(hostParam, sgParam)
}

// Provider specific functions

// Build ...
func (w ErrorTraceProvider) Build(something map[string]interface{}) (p Provider, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:Build", w.Name))
	return w.InnerProvider.Build(something)
}

// ListImages ...
func (w ErrorTraceProvider) ListImages(all bool) (images []abstract.Image, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListImages", w.Name))
	return w.InnerProvider.ListImages(all)
}

// ListTemplates ...
func (w ErrorTraceProvider) ListTemplates(all bool) (templates []abstract.HostTemplate, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListTemplates", w.Name))
	return w.InnerProvider.ListTemplates(all)
}

// InspectAuthenticationOptions ...
func (w ErrorTraceProvider) GetAuthenticationOptions() (cfg Config, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetAuthenticationOptions", w.Name))

	return w.InnerProvider.GetAuthenticationOptions()
}

// InspectConfigurationOptions ...
func (w ErrorTraceProvider) GetConfigurationOptions() (cfg Config, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetConfigurationOptions", w.Name))
	return w.InnerProvider.GetConfigurationOptions()
}

// InspectName ...
func (w ErrorTraceProvider) GetName() string {
	return w.InnerProvider.GetName()
}

// InspectTenantParameters ...
func (w ErrorTraceProvider) GetTenantParameters() map[string]interface{} {
	return w.InnerProvider.GetTenantParameters()
}

// Stack specific functions

// NewErrorTraceProvider ...
func NewErrorTraceProvider(innerProvider Provider, name string) *ErrorTraceProvider {
	ep := &ErrorTraceProvider{InnerProvider: innerProvider, Name: name}

	var _ Provider = ep

	return ep
}

// ListAvailabilityZones ...
func (w ErrorTraceProvider) ListAvailabilityZones() (zones map[string]bool, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListAvailabilityZones", w.Name))
	return w.InnerProvider.ListAvailabilityZones()
}

// ListRegions ...
func (w ErrorTraceProvider) ListRegions() (regions []string, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListRegions", w.Name))
	return w.InnerProvider.ListRegions()
}

// InspectImage ...
func (w ErrorTraceProvider) InspectImage(id string) (images *abstract.Image, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetImage", w.Name))
	return w.InnerProvider.InspectImage(id)
}

// InspectTemplate ...
func (w ErrorTraceProvider) InspectTemplate(id string) (templates *abstract.HostTemplate, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetTemplate", w.Name))
	return w.InnerProvider.InspectTemplate(id)
}

// CreateKeyPair ...
func (w ErrorTraceProvider) CreateKeyPair(name string) (pairs *abstract.KeyPair, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:CreateKeyPair", w.Name))
	return w.InnerProvider.CreateKeyPair(name)
}

// InspectKeyPair ...
func (w ErrorTraceProvider) InspectKeyPair(id string) (pairs *abstract.KeyPair, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetKeyPair", w.Name))
	return w.InnerProvider.InspectKeyPair(id)
}

// ListKeyPairs ...
func (w ErrorTraceProvider) ListKeyPairs() (pairs []abstract.KeyPair, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListKeyPairs", w.Name))
	return w.InnerProvider.ListKeyPairs()
}

// DeleteKeyPair ...
func (w ErrorTraceProvider) DeleteKeyPair(id string) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteKeyPair", w.Name))
	return w.InnerProvider.DeleteKeyPair(id)
}

// CreateNetwork ...
func (w ErrorTraceProvider) CreateNetwork(req abstract.NetworkRequest) (net *abstract.Network, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:CreateNetwork", w.Name))
	return w.InnerProvider.CreateNetwork(req)
}

// InspectNetwork ...
func (w ErrorTraceProvider) InspectNetwork(id string) (net *abstract.Network, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetNetwork", w.Name))
	return w.InnerProvider.InspectNetwork(id)
}

// InspectNetworkByName ...
func (w ErrorTraceProvider) InspectNetworkByName(name string) (net *abstract.Network, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetNetworkByName", w.Name))
	return w.InnerProvider.InspectNetworkByName(name)
}

// ListNetworks ...
func (w ErrorTraceProvider) ListNetworks() (net []*abstract.Network, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListNetworks", w.Name))
	return w.InnerProvider.ListNetworks()
}

// DeleteNetwork ...
func (w ErrorTraceProvider) DeleteNetwork(id string) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteNetwork", w.Name))
	return w.InnerProvider.DeleteNetwork(id)
}

// CreateVIP ...
func (w ErrorTraceProvider) CreateVIP(networkID string, description string) (_ *abstract.VirtualIP, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:CreateVIP", w.Name))
	return w.InnerProvider.CreateVIP(networkID, description)
}

// AddPublicIPToVIP adds a public IP to VIP
func (w ErrorTraceProvider) AddPublicIPToVIP(vip *abstract.VirtualIP) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:AddPublicIPToVIP", w.Name))
	return w.InnerProvider.AddPublicIPToVIP(vip)
}

// BindHostToVIP makes the host passed as parameter an allowed "target" of the VIP
func (w ErrorTraceProvider) BindHostToVIP(vip *abstract.VirtualIP, hostID string) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:BindHostToVIP", w.Name))
	return w.InnerProvider.BindHostToVIP(vip, hostID)
}

// UnbindHostFromVIP removes the bind between the VIP and a host
func (w ErrorTraceProvider) UnbindHostFromVIP(vip *abstract.VirtualIP, hostID string) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:UnbindHostFromVIP", w.Name))
	return w.InnerProvider.UnbindHostFromVIP(vip, hostID)
}

// DeleteVIP deletes the port corresponding to the VIP
func (w ErrorTraceProvider) DeleteVIP(vip *abstract.VirtualIP) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteVIP", w.Name))
	return w.InnerProvider.DeleteVIP(vip)
}

// CreateHost ...
func (w ErrorTraceProvider) CreateHost(request abstract.HostRequest) (_ *abstract.HostFull, _ *userdata.Content, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:CreateHost", w.Name))
	return w.InnerProvider.CreateHost(request)
}

// InspectHost ...
func (w ErrorTraceProvider) InspectHost(something stacks.HostParameter) (_ *abstract.HostFull, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:InspectHost", w.Name))
	return w.InnerProvider.InspectHost(something)
}

// InspectHostByName ...
func (w ErrorTraceProvider) InspectHostByName(name string) (_ *abstract.HostCore, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetHostByName", w.Name))
	return w.InnerProvider.InspectHostByName(name)
}

// InspectHostState ...
func (w ErrorTraceProvider) GetHostState(something stacks.HostParameter) (_ hoststate.Enum, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetHostState", w.Name))
	return w.InnerProvider.GetHostState(something)
}

// ListHosts ...
func (w ErrorTraceProvider) ListHosts(b bool) (_ abstract.HostList, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListHosts", w.Name))
	return w.InnerProvider.ListHosts(b)
}

// DeleteHost ...
func (w ErrorTraceProvider) DeleteHost(id stacks.HostParameter) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteHost", w.Name))
	return w.InnerProvider.DeleteHost(id)
}

// StopHost ...
func (w ErrorTraceProvider) StopHost(id stacks.HostParameter) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:StopHost", w.Name))
	return w.InnerProvider.StopHost(id)
}

// StartHost ...
func (w ErrorTraceProvider) StartHost(id stacks.HostParameter) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:StartHost", w.Name))
	return w.InnerProvider.StartHost(id)
}

// RebootHost ...
func (w ErrorTraceProvider) RebootHost(id stacks.HostParameter) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:RebootHost", w.Name))
	return w.InnerProvider.RebootHost(id)
}

// ResizeHost ...
func (w ErrorTraceProvider) ResizeHost(id stacks.HostParameter, request abstract.HostSizingRequirements) (_ *abstract.HostFull, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ResizeHost", w.Name))
	return w.InnerProvider.ResizeHost(id, request)
}

// CreateVolume ...
func (w ErrorTraceProvider) CreateVolume(request abstract.VolumeRequest) (_ *abstract.Volume, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:CreateVolume", w.Name))
	return w.InnerProvider.CreateVolume(request)
}

// InspectVolume ...
func (w ErrorTraceProvider) InspectVolume(id string) (_ *abstract.Volume, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetVolume", w.Name))
	return w.InnerProvider.InspectVolume(id)
}

// ListVolumes ...
func (w ErrorTraceProvider) ListVolumes() (_ []abstract.Volume, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListVolumes", w.Name))
	return w.InnerProvider.ListVolumes()
}

// DeleteVolume ...
func (w ErrorTraceProvider) DeleteVolume(id string) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteVolume", w.Name))
	return w.InnerProvider.DeleteVolume(id)
}

// CreateVolumeAttachment ...
func (w ErrorTraceProvider) CreateVolumeAttachment(request abstract.VolumeAttachmentRequest) (_ string, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:CreateVolumeAttachment", w.Name))
	return w.InnerProvider.CreateVolumeAttachment(request)
}

// InspectVolumeAttachment ...
func (w ErrorTraceProvider) InspectVolumeAttachment(serverID, id string) (_ *abstract.VolumeAttachment, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:GetVolumeAttachment", w.Name))
	return w.InnerProvider.InspectVolumeAttachment(serverID, id)
}

// ListVolumeAttachments ...
func (w ErrorTraceProvider) ListVolumeAttachments(serverID string) (_ []abstract.VolumeAttachment, err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:ListVolumeAttachments", w.Name))
	return w.InnerProvider.ListVolumeAttachments(serverID)
}

// DeleteVolumeAttachment ...
func (w ErrorTraceProvider) DeleteVolumeAttachment(serverID, id string) (err fail.Error) {
	defer func(prefix string) {
		if err != nil {
			logrus.Debugf("%s : Intercepted error: %v", prefix, err)
		}
	}(fmt.Sprintf("%s:DeleteVolumeAttachment", w.Name))
	return w.InnerProvider.DeleteVolumeAttachment(serverID, id)
}

// InspectCapabilities ...
func (w ErrorTraceProvider) GetCapabilities() Capabilities {
	return w.InnerProvider.GetCapabilities()
}
