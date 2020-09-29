package providers

import (
	"time"

	"github.com/CS-SI/SafeScale/lib/server/iaas/stacks"
	"github.com/CS-SI/SafeScale/lib/server/iaas/userdata"
	"github.com/CS-SI/SafeScale/lib/server/resources/abstract"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/hoststate"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
	"github.com/CS-SI/SafeScale/lib/utils/retry"
	"github.com/CS-SI/SafeScale/lib/utils/temporal"
)

// RetryProvider ...
type RetryProvider WrappedProvider

func (w RetryProvider) ListSecurityGroups() (res []*abstract.SecurityGroup, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListSecurityGroups()
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) CreateSecurityGroup(
	name string, description string, rules []abstract.SecurityGroupRule,
) (res *abstract.SecurityGroup, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.CreateSecurityGroup(name, description, rules)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) InspectSecurityGroup(sgParam stacks.SecurityGroupParameter) (
	res *abstract.SecurityGroup, err fail.Error,
) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectSecurityGroup(sgParam)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) ClearSecurityGroup(sgParam stacks.SecurityGroupParameter) (res *abstract.SecurityGroup, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ClearSecurityGroup(sgParam)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) DeleteSecurityGroup(sgParam stacks.SecurityGroupParameter) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.DeleteSecurityGroup(sgParam)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

func (w RetryProvider) AddRuleToSecurityGroup(
	sgParam stacks.SecurityGroupParameter, rule abstract.SecurityGroupRule,
) (res *abstract.SecurityGroup, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.AddRuleToSecurityGroup(sgParam, rule)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) DeleteRuleFromSecurityGroup(
	sgParam stacks.SecurityGroupParameter, ruleID string,
) (res *abstract.SecurityGroup, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.DeleteRuleFromSecurityGroup(sgParam, ruleID)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) WaitHostReady(hostParam stacks.HostParameter, timeout time.Duration) (
	res *abstract.HostCore, err fail.Error,
) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.WaitHostReady(hostParam, timeout)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) BindSecurityGroupToHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.BindSecurityGroupToHost(hostParam, sgParam)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

func (w RetryProvider) UnbindSecurityGroupFromHost(
	hostParam stacks.HostParameter, sgParam stacks.SecurityGroupParameter,
) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.UnbindSecurityGroupFromHost(hostParam, sgParam)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

func (w RetryProvider) CreateVIP(first string, second string) (res *abstract.VirtualIP, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.CreateVIP(first, second)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) AddPublicIPToVIP(res *abstract.VirtualIP) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.AddPublicIPToVIP(res)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

func (w RetryProvider) BindHostToVIP(vip *abstract.VirtualIP, hostID string) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.BindHostToVIP(vip, hostID)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

func (w RetryProvider) UnbindHostFromVIP(vip *abstract.VirtualIP, hostID string) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.UnbindHostFromVIP(vip, hostID)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err
				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

func (w RetryProvider) DeleteVIP(vip *abstract.VirtualIP) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.DeleteVIP(vip)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

func (w RetryProvider) GetCapabilities() Capabilities {
	return w.InnerProvider.GetCapabilities()
}

func (w RetryProvider) GetTenantParameters() map[string]interface{} {
	return w.InnerProvider.GetTenantParameters()
}

// Provider specific functions

func (w RetryProvider) Build(something map[string]interface{}) (p Provider, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			p, err = w.InnerProvider.Build(something)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return p, retryErr
	}

	return p, err
}

func (w RetryProvider) ListImages(all bool) (res []abstract.Image, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListImages(all)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) ListTemplates(all bool) (res []abstract.HostTemplate, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListTemplates(all)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

func (w RetryProvider) GetAuthenticationOptions() (Config, fail.Error) {
	return w.InnerProvider.GetAuthenticationOptions()
}

func (w RetryProvider) GetConfigurationOptions() (Config, fail.Error) {
	return w.InnerProvider.GetConfigurationOptions()
}

func (w RetryProvider) GetName() string {
	return w.InnerProvider.GetName()
}

// Stack specific functions

// NewRetryProvider ...
func NewRetryProvider(InnerProvider Provider, name string) *RetryProvider {
	rp := &RetryProvider{InnerProvider: InnerProvider, Name: name}
	var _ Provider = rp
	return rp
}

// ListAvailabilityZones ...
func (w RetryProvider) ListAvailabilityZones() (res map[string]bool, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListAvailabilityZones()
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// ListRegions ...
func (w RetryProvider) ListRegions() (res []string, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListRegions()
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectImage ...
func (w RetryProvider) InspectImage(id string) (res *abstract.Image, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectImage(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectTemplate ...
func (w RetryProvider) InspectTemplate(id string) (res *abstract.HostTemplate, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectTemplate(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// CreateKeyPair ...
func (w RetryProvider) CreateKeyPair(name string) (kp *abstract.KeyPair, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			kp, err = w.InnerProvider.CreateKeyPair(name)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return kp, retryErr
	}

	return kp, err
}

// InspectKeyPair ...
func (w RetryProvider) InspectKeyPair(id string) (kp *abstract.KeyPair, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			kp, err = w.InnerProvider.InspectKeyPair(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return kp, retryErr
	}

	return kp, err
}

// ListKeyPairs ...
func (w RetryProvider) ListKeyPairs() (res []abstract.KeyPair, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListKeyPairs()
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// DeleteKeyPair ...
func (w RetryProvider) DeleteKeyPair(id string) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.DeleteKeyPair(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

// CreateNetwork ...
func (w RetryProvider) CreateNetwork(req abstract.NetworkRequest) (res *abstract.Network, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.CreateNetwork(req)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectNetwork ...
func (w RetryProvider) InspectNetwork(id string) (res *abstract.Network, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectNetwork(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectNetworkByName ...
func (w RetryProvider) InspectNetworkByName(name string) (res *abstract.Network, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectNetworkByName(name)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// ListNetworks ...
func (w RetryProvider) ListNetworks() (res []*abstract.Network, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListNetworks()
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// DeleteNetwork ...
func (w RetryProvider) DeleteNetwork(id string) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.DeleteNetwork(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

// CreateHost ...
func (w RetryProvider) CreateHost(request abstract.HostRequest) (
	res *abstract.HostFull, data *userdata.Content, err fail.Error,
) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, data, err = w.InnerProvider.CreateHost(request)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, data, retryErr
	}

	return res, data, err
}

// InspectHost ...
func (w RetryProvider) InspectHost(something stacks.HostParameter) (res *abstract.HostFull, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectHost(something)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectHostByName ...
func (w RetryProvider) InspectHostByName(name string) (res *abstract.HostCore, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectHostByName(name)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectHostState ...
func (w RetryProvider) GetHostState(something stacks.HostParameter) (res hoststate.Enum, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.GetHostState(something)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// ListHosts ...
func (w RetryProvider) ListHosts(b bool) (res abstract.HostList, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListHosts(b)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// DeleteHost ...
func (w RetryProvider) DeleteHost(id stacks.HostParameter) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.DeleteHost(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

// StopHost ...
func (w RetryProvider) StopHost(id stacks.HostParameter) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.StopHost(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

// StartHost ...
func (w RetryProvider) StartHost(id stacks.HostParameter) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.StartHost(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

// RebootHost ...
func (w RetryProvider) RebootHost(id stacks.HostParameter) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.RebootHost(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

// ResizeHost ...
func (w RetryProvider) ResizeHost(id stacks.HostParameter, request abstract.HostSizingRequirements) (res *abstract.HostFull, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ResizeHost(id, request)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// CreateVolume ...
func (w RetryProvider) CreateVolume(request abstract.VolumeRequest) (res *abstract.Volume, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.CreateVolume(request)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectVolume ...
func (w RetryProvider) InspectVolume(id string) (res *abstract.Volume, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectVolume(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// ListVolumes ...
func (w RetryProvider) ListVolumes() (res []abstract.Volume, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListVolumes()
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// DeleteVolume ...
func (w RetryProvider) DeleteVolume(id string) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.DeleteVolume(id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}

// CreateVolumeAttachment ...
func (w RetryProvider) CreateVolumeAttachment(request abstract.VolumeAttachmentRequest) (res string, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.CreateVolumeAttachment(request)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// InspectVolumeAttachment ...
func (w RetryProvider) InspectVolumeAttachment(serverID, id string) (res *abstract.VolumeAttachment, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.InspectVolumeAttachment(serverID, id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// ListVolumeAttachments ...
func (w RetryProvider) ListVolumeAttachments(serverID string) (res []abstract.VolumeAttachment, err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			res, err = w.InnerProvider.ListVolumeAttachments(serverID)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return res, retryErr
	}

	return res, err
}

// DeleteVolumeAttachment ...
func (w RetryProvider) DeleteVolumeAttachment(serverID, id string) (err fail.Error) {
	retryErr := retry.WhileUnsuccessful(
		func() error {
			err = w.InnerProvider.DeleteVolumeAttachment(serverID, id)
			if err != nil {
				switch err.(type) {
				case *fail.ErrTimeout:
					return err

				case *fail.ErrNetworkIssue:
					return err
				default:
					return nil
				}
			}
			return nil
		},
		0,
		temporal.GetContextTimeout(),
	)
	if retryErr != nil {
		return retryErr
	}

	return err
}
