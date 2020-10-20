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

package operations

import (
	"fmt"
	"github.com/CS-SI/SafeScale/lib/server/iaas/stacks"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/securitygroupstate"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/subnetproperty"
	"os"
	"os/user"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/lib/protocol"
	"github.com/CS-SI/SafeScale/lib/server/iaas"
	"github.com/CS-SI/SafeScale/lib/server/iaas/userdata"
	"github.com/CS-SI/SafeScale/lib/server/resources"
	"github.com/CS-SI/SafeScale/lib/server/resources/abstract"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/featuretargettype"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/hostproperty"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/hoststate"
	"github.com/CS-SI/SafeScale/lib/server/resources/enums/installmethod"
	"github.com/CS-SI/SafeScale/lib/server/resources/operations/converters"
	propertiesv1 "github.com/CS-SI/SafeScale/lib/server/resources/properties/v1"
	propertiesv2 "github.com/CS-SI/SafeScale/lib/server/resources/properties/v2"
	"github.com/CS-SI/SafeScale/lib/system"
	"github.com/CS-SI/SafeScale/lib/utils/cli/enums/outputs"
	"github.com/CS-SI/SafeScale/lib/utils/concurrency"
	"github.com/CS-SI/SafeScale/lib/utils/data"
	"github.com/CS-SI/SafeScale/lib/utils/debug"
	"github.com/CS-SI/SafeScale/lib/utils/debug/tracing"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
	"github.com/CS-SI/SafeScale/lib/utils/retry"
	"github.com/CS-SI/SafeScale/lib/utils/retry/enums/verdict"
	"github.com/CS-SI/SafeScale/lib/utils/serialize"
	"github.com/CS-SI/SafeScale/lib/utils/strprocess"
	"github.com/CS-SI/SafeScale/lib/utils/temporal"
)

const (
	// hostsFolderName is the technical name of the container used to store networks info
	hostsFolderName = "hosts"

	defaultHostSecurityGroupNamePattern = "host_%s.%s.%s" // host_<hostname>.<subnet name>.<network name> should be unique across a tenant
)

// host ...
// follows interface resources.Host
type host struct {
	*core

	installMethods                map[uint8]installmethod.Enum
	privateIP, publicIP, accessIP string
	sshProfile                    *system.SSHConfig
}

// NewHost ...
func NewHost(svc iaas.Service) (*host, fail.Error) {
	if svc == nil {
		return nil, fail.InvalidParameterError("svc", "cannot be nil")
	}

	coreInstance, xerr := newCore(svc, "host", hostsFolderName, &abstract.HostCore{})
	if xerr != nil {
		return nil, xerr
	}

	return &host{core: coreInstance}, nil
}

// nullHost returns a *host corresponding to NullValue
func nullHost() *host {
	return &host{core: nullCore()}
}

// LoadHost ...
func LoadHost(task concurrency.Task, svc iaas.Service, ref string) (_ resources.Host, xerr fail.Error) {
	if task.IsNull() {
		return nullHost(), fail.InvalidParameterError("task", "cannot be nil")
	}
	if svc == nil {
		return nullHost(), fail.InvalidParameterError("svc", "cannot be nil")
	}
	if ref == "" {
		return nullHost(), fail.InvalidParameterError("ref", "cannot be empty string")
	}
	defer fail.OnPanic(&xerr)

	rh, xerr := NewHost(svc)
	if xerr != nil {
		return nullHost(), xerr
	}

	// TODO: core.Read() does not check communication failure, side effect of limitations of Stow (waiting for stow replacement by rclone)
	if xerr = rh.Read(task, ref); xerr != nil {
		switch xerr.(type) {
		case *fail.ErrNotFound:
			// rewrite NotFoundError, user does not bother about metadata stuff
			return nullHost(), fail.NotFoundError("failed to find Host '%s'", ref)
		default:
			return nullHost(), xerr
		}
	}

	if xerr = rh.upgradeIfNeeded(task); xerr != nil {
		return nil, fail.Wrap(xerr, "failed to upgrade Host metadata")
	}

	// (re)cache information only if there was no error
	return rh, rh.cacheAccessInformation(task)
}

// upgradeIfNeeded upgrades Host properties if needed
func (rh *host) upgradeIfNeeded(task concurrency.Task) fail.Error {
	rh.SafeLock(task)
	defer rh.SafeUnlock(task)

	return rh.Alter(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {

		// upgrade hostproperty.NetworkV1 to hostproperty.NetworkV2
		if !props.Lookup(hostproperty.NetworkV2) {
			xerr := props.Alter(task, hostproperty.NetworkV1, func(clonable data.Clonable) fail.Error {
				hnV1, ok := clonable.(*propertiesv1.HostNetwork)
				if !ok {
					return fail.InconsistentError("'*propertiesv1.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}
				innerXErr := props.Alter(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
					hnV2, ok := clonable.(*propertiesv2.HostNetworking)
					if !ok {
						return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
					}
					hnV2.DefaultSubnetID = hnV1.DefaultNetworkID
					hnV2.IPv4Addresses = hnV1.IPv4Addresses
					hnV2.IPv6Addresses = hnV1.IPv6Addresses
					hnV2.IsGateway = hnV1.IsGateway
					hnV2.PublicIPv4 = hnV1.PublicIPv4
					hnV2.PublicIPv6 = hnV1.PublicIPv6
					hnV2.SubnetsByID = hnV1.NetworksByID
					hnV2.SubnetsByName = hnV1.NetworksByName
					return nil
				})
				if innerXErr != nil {
					return innerXErr
				}
				// FIXME: clean old property or leave it ? will differ from v2 through time if Subnets are added for example
				//hnV1 = &propertiesv1.HostNetworking{}
				return nil
			})
			if xerr != nil {
				return xerr
			}
		}

		return nil
	})
}

// cacheAccessInformation loads in cache SSH configuration to access host; this information will not change over time
func (rh *host) cacheAccessInformation(task concurrency.Task) fail.Error {
	svc := rh.GetService()

	rh.SafeLock(task)
	defer rh.SafeUnlock(task)

	return rh.Inspect(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
		var primaryGatewayConfig, secondaryGatewayConfig *system.SSHConfig

		ahc, ok := clonable.(*abstract.HostCore)
		if !ok {
			return fail.InconsistentError("'*abstract.HostCore' expected, '%s' provided", reflect.TypeOf(clonable).String())
		}

		innerXErr := props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hnV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}

			if len(hnV2.IPv4Addresses) > 0 {
				rh.privateIP = hnV2.IPv4Addresses[hnV2.DefaultSubnetID]
				if rh.privateIP == "" {
					rh.privateIP = hnV2.IPv6Addresses[hnV2.DefaultSubnetID]
				}
			}
			rh.publicIP = hnV2.PublicIPv4
			if rh.publicIP == "" {
				rh.publicIP = hnV2.PublicIPv6
			}
			if rh.publicIP != "" {
				rh.accessIP = rh.publicIP
			} else {
				rh.accessIP = rh.privateIP
			}

			if !hnV2.IsGateway {
				rs, xerr := LoadSubnet(task, svc, "", hnV2.DefaultSubnetID)
				if xerr != nil {
					return xerr
				}
				rgw, xerr := rs.GetGateway(task, true)
				if xerr != nil {
					return xerr
				}
				gwErr := rgw.Inspect(task, func(clonable data.Clonable, _ *serialize.JSONProperties) fail.Error {
					gwahc, ok := clonable.(*abstract.HostCore)
					if !ok {
						return fail.InconsistentError("'*abstract.HostCore' expected, '%s' provided", reflect.TypeOf(clonable).String())
					}
					ip := rgw.(*host).getAccessIP(task)
					primaryGatewayConfig = &system.SSHConfig{
						PrivateKey: gwahc.PrivateKey,
						Port:       22,
						Host:       ip,
						User:       abstract.DefaultUser,
					}
					return nil
				})
				if gwErr != nil {
					return gwErr
				}

				// Secondary gateway may not exist...
				rgw, xerr = rs.GetGateway(task, false)
				if xerr != nil {
					if _, ok := xerr.(*fail.ErrNotFound); !ok {
						return xerr
					}
				} else {
					gwErr = rgw.Inspect(task, func(clonable data.Clonable, _ *serialize.JSONProperties) fail.Error {
						gwahc, ok := clonable.(*abstract.HostCore)
						if !ok {
							return fail.InconsistentError("'*abstract.HostCore' expected, '%s' provided", reflect.TypeOf(clonable).String())
						}
						secondaryGatewayConfig = &system.SSHConfig{
							PrivateKey: gwahc.PrivateKey,
							Port:       22,
							Host:       rgw.(*host).getAccessIP(task),
							User:       abstract.DefaultUser,
						}
						return nil
					})
					if gwErr != nil {
						return gwErr
					}
				}
			}
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}
		rh.sshProfile = &system.SSHConfig{
			Port:                   22,
			Host:                   rh.accessIP,
			User:                   abstract.DefaultUser,
			PrivateKey:             ahc.PrivateKey,
			GatewayConfig:          primaryGatewayConfig,
			SecondaryGatewayConfig: secondaryGatewayConfig,
		}
		return nil
	})
}

// IsNull tests if instance is nil or empty
func (rh *host) IsNull() bool {
	return rh == nil || rh.core.IsNull()
}

// Browse walks through host folder and executes a callback for each entries
func (rh host) Browse(task concurrency.Task, callback func(*abstract.HostCore) fail.Error) (xerr fail.Error) {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task.IsNull() {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	if callback == nil {
		return fail.InvalidParameterError("callback", "cannot be nil")
	}

	return rh.core.BrowseFolder(task, func(buf []byte) fail.Error {
		ahc := abstract.NewHostCore()
		if xerr = ahc.Deserialize(buf); xerr != nil {
			return xerr
		}
		return callback(ahc)
	})
}

// ForceGetState returns the current state of the provider host
func (rh *host) ForceGetState(task concurrency.Task) (state hoststate.Enum, _ fail.Error) {
	state = hoststate.UNKNOWN
	if rh.IsNull() {
		return state, fail.InvalidInstanceError()
	}
	if task.IsNull() {
		return state, fail.InvalidParameterError("task", "cannot be nil")
	}

	if xerr := rh.Reload(task); xerr != nil {
		return state, xerr
	}
	xerr := rh.Inspect(task, func(clonable data.Clonable, _ *serialize.JSONProperties) fail.Error {
		ahc, ok := clonable.(*abstract.HostCore)
		if !ok {
			return fail.InconsistentError("'*abstract.HostCore' expected, '%s' provided", reflect.TypeOf(clonable).String())
		}
		state = ahc.LastState
		return nil

	})
	return state, xerr
}

// Reload reloads host from metadata and current host state on provider state
func (rh *host) Reload(task concurrency.Task) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task.IsNull() {
		return fail.InvalidParameterError("task", "cannot be nil")
	}

	// Read data from metadata storage
	hostID := rh.GetID()
	xerr := retry.WhileUnsuccessfulDelay1Second(
		func() error {
			return rh.Read(task, hostID)
		},
		10*time.Second,
	)
	if xerr != nil {
		// If retry timed out, log it and return error ErrNotFound
		if _, ok := xerr.(*retry.ErrTimeout); ok {
			xerr = fail.NotFoundError("metadata of host '%s' not found; host deleted?", hostID)
		}
		return xerr
	}

	// Request host inspection from provider
	ahf, xerr := rh.GetService().InspectHost(rh.GetID())
	if xerr != nil {
		return xerr
	}

	// Updates the host metadata
	xerr = rh.Alter(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
		ahc, ok := clonable.(*abstract.HostCore)
		if !ok {
			return fail.InconsistentError("'*abstract.HostCore' expected, '%s' received", reflect.TypeOf(clonable).String())
		}
		changed := false
		if ahc.LastState != ahf.CurrentState {
			ahc.LastState = ahf.CurrentState
			changed = true
		}

		innerXErr := props.Alter(task, hostproperty.SizingV1, func(clonable data.Clonable) fail.Error {
			hostSizingV1, ok := clonable.(*propertiesv1.HostSizing)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostSizing' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			allocated := converters.HostEffectiveSizingFromAbstractToPropertyV1(ahf.Sizing)
			// FIXME: how to compare the 2 structs ?
			if allocated != hostSizingV1.AllocatedSize {
				hostSizingV1.AllocatedSize = allocated
				changed = true
			}
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}

		// Updates host property propertiesv1.HostNetworking
		innerXErr = props.Alter(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hnV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			_ = hnV2.Replace(converters.HostNetworkingFromAbstractToPropertyV2(*ahf.Networking))
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}
		if !changed {
			return fail.AlteredNothingError()
		}
		return nil
	})
	if xerr != nil {
		switch xerr.(type) {
		case *fail.ErrAlteredNothing:
			return nil
		default:
			return xerr
		}
	}

	return rh.cacheAccessInformation(task)
}

// GetState returns the last known state of the host, without forced inspect
func (rh host) GetState(task concurrency.Task) (state hoststate.Enum) {
	state = hoststate.UNKNOWN
	if rh.IsNull() || task.IsNull() {
		return state
	}

	_ = rh.Inspect(task, func(clonable data.Clonable, _ *serialize.JSONProperties) fail.Error {
		ahc, ok := clonable.(*abstract.HostCore)
		if !ok {
			return fail.InconsistentError("'*abstract.HostCore' expected, '%s' provided", reflect.TypeOf(clonable).String())
		}
		state = ahc.LastState
		return nil
	})
	return state
}

// Create creates a new host and its metadata
// If the metadata is already carrying a host, returns fail.ErrNotAvailable
func (rh *host) Create(task concurrency.Task, hostReq abstract.HostRequest, hostDef abstract.HostSizingRequirements) (_ *userdata.Content, xerr fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task.IsNull() {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}
	hostname := rh.GetName()
	if hostname != "" {
		return nil, fail.NotAvailableError("already carrying host '%s'", hostname)
	}

	tracer := debug.NewTracer(task, tracing.ShouldTrace("resources.host"), "(%s)", hostReq.ResourceName).WithStopwatch().Entering()
	defer tracer.Exiting()
	//defer fail.OnExitLogError(&xerr, "failed to create host")
	defer fail.OnPanic(&xerr)

	svc := rh.GetService()

	// Check if host exists and is managed bySafeScale
	if _, xerr = LoadHost(task, svc, hostReq.ResourceName); xerr != nil {
		switch xerr.(type) {
		case *fail.ErrNotFound:
		// continue
		default:
			return nil, fail.Wrap(xerr, "failed to check if host '%s' already exists", hostReq.ResourceName)
		}
	} else {
		return nil, fail.DuplicateError("'%s' already exists", hostReq.ResourceName)
	}

	// Check if host exists but is not managed by SafeScale
	if _, xerr = svc.InspectHostByName(hostReq.ResourceName); xerr != nil {
		switch xerr.(type) {
		case *fail.ErrNotFound:
			// continue
		default:
			return nil, fail.Wrap(xerr, "failed to check if host resource name '%s' is already used", hostReq.ResourceName)
		}
	} else {
		return nil, fail.DuplicateError("'%s' already exists (but not managed by SafeScale)", hostReq.ResourceName)
	}

	// If TemplateID is not explicitly provided, search the appropriate template to satisfy 'hostDef'
	if hostReq.TemplateID == "" {
		hostReq.TemplateID, xerr = rh.findTemplateID(hostDef)
		if xerr != nil {
			return nil, xerr
		}
	}

	// identify default Subnet
	var defaultSubnet resources.Subnet
	if len(hostReq.Subnets) > 0 {
		// By convention, default subnet is the first of the list
		as := hostReq.Subnets[0]
		if defaultSubnet, xerr = LoadSubnet(task, svc, "", as.ID); xerr != nil {
			return nil, xerr
		}
		if hostReq.DefaultRouteIP == "" {
			hostReq.DefaultRouteIP = defaultSubnet.(*subnet).getDefaultRouteIP(task)
		}
	} else {
		if defaultSubnet, _, xerr = getOrCreateDefaultSubnet(task, svc); xerr != nil {
			return nil, xerr
		}
		xerr = defaultSubnet.Inspect(task, func(clonable data.Clonable, _ *serialize.JSONProperties) fail.Error {
			as, ok := clonable.(*abstract.Subnet)
			if !ok {
				return fail.InconsistentError("'*abstract.Networking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			hostReq.Subnets = append(hostReq.Subnets, as)
			return nil
		})
		if xerr != nil {
			return nil, xerr
		}
	}

	// If hostReq.ImageID is not explicitly defined, find an image ID corresponding to the content of hostDef.Image
	if hostReq.ImageID == "" {
		hostReq.ImageID, xerr = rh.findImageID(&hostDef)
		if xerr != nil {
			return nil, fail.Wrap(xerr, "failed to find image to use on compute resource")
		}
	}

	hostReq.Password = "safescale" // VPL:for debugging purpose, remove if you see this!
	ahf, userdataContent, xerr := svc.CreateHost(hostReq)
	if xerr != nil {
		if _, ok := xerr.(*fail.ErrInvalidRequest); ok {
			return nil, xerr
		}
		return nil, fail.Wrap(xerr, "failed to create compute resource '%s'", hostReq.ResourceName)
	}

	defer func() {
		if xerr != nil && !hostReq.KeepOnFailure {
			derr := svc.DeleteHost(ahf.Core.ID)
			if derr != nil {
				logrus.Errorf("cleaning up after failure, failed to delete host '%s': %v", ahf.Core.Name, derr)
				_ = xerr.AddConsequence(derr)
			}
		}
	}()

	// Creates metadata early to "reserve" host name
	if xerr = rh.Carry(task, ahf.Core); xerr != nil {
		return nil, xerr
	}

	defer func() {
		if xerr != nil && !hostReq.KeepOnFailure {
			derr := rh.core.Delete(task)
			if derr != nil {
				logrus.Errorf("cleaning up after failure, failed to delete host '%s' metadata: %v", ahf.Core.Name, derr)
				_ = xerr.AddConsequence(derr)
			}
		}
	}()

	xerr = rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		innerXErr := props.Alter(task, hostproperty.SizingV1, func(clonable data.Clonable) fail.Error {
			hostSizingV1, ok := clonable.(*propertiesv1.HostSizing)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSizing' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			hostSizingV1.AllocatedSize = converters.HostEffectiveSizingFromAbstractToPropertyV1(ahf.Sizing)
			hostSizingV1.RequestedSize = converters.HostSizingRequirementsFromAbstractToPropertyV1(hostDef)
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}

		// Sets host extension DescriptionV1
		innerXErr = props.Alter(task, hostproperty.DescriptionV1, func(clonable data.Clonable) fail.Error {
			hostDescriptionV1, ok := clonable.(*propertiesv1.HostDescription)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostDescription' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			_ = hostDescriptionV1.Replace(converters.HostDescriptionFromAbstractToPropertyV1(*ahf.Description))
			creator := ""
			hostname, _ := os.Hostname()
			if curUser, err := user.Current(); err == nil {
				creator = curUser.Username
				if hostname != "" {
					creator += "@" + hostname
				}
				if curUser.Name != "" {
					creator += " (" + curUser.Name + ")"
				}
			} else {
				creator = "unknown@" + hostname
			}
			hostDescriptionV1.Creator = creator
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}

		// Updates host property propertiesv2.HostNetworking
		// FIXME: introduce a NetworkV2 with correctly named fields (Networking -> Subnet, ...)
		innerXErr = props.Alter(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hnV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			_ = hnV2.Replace(converters.HostNetworkingFromAbstractToPropertyV2(*ahf.Networking))
			hnV2.DefaultSubnetID = defaultSubnet.GetID()
			hnV2.IsGateway = hostReq.IsGateway
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}

		// Updates properties in metadata
		return rh.setSecurityGroups(task, hostReq, defaultSubnet)
	})
	if xerr != nil {
		return nil, xerr
	}
	defer rh.onFailureUndoSetSecurityGroups(task, &xerr, hostReq.KeepOnFailure)

	if xerr = rh.cacheAccessInformation(task); xerr != nil {
		return nil, xerr
	}

	logrus.Infof("Compute resource created: '%s'", rh.GetName())

	// A host claimed ready by a Cloud provider is not necessarily ready
	// to be used until ssh service is up and running. So we wait for it before
	// claiming host is created
	logrus.Infof("Waiting start of SSH service on remote host '%s' ...", rh.GetName())

	// FIXME: configurable timeout here
	status, xerr := rh.waitInstallPhase(task, userdata.PHASE1_INIT, time.Duration(0))
	if xerr != nil {
		if _, ok := xerr.(*fail.ErrTimeout); ok {
			return nil, fail.Wrap(xerr, "timeout creating a host")
		}
		if abstract.IsProvisioningError(xerr) {
			logrus.Errorf("%+v", xerr)
			return nil, fail.Wrap(xerr, "error provisioning the new host, please check safescaled logs", rh.GetName())
		}
		return nil, xerr
	}

	xerr = rh.Alter(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
		// update host system property
		return props.Alter(task, hostproperty.SystemV1, func(clonable data.Clonable) fail.Error {
			systemV1, ok := clonable.(*propertiesv1.HostSystem)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSystem' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			parts := strings.Split(status, ",")
			systemV1.Type = parts[1]
			systemV1.Flavor = parts[2]
			systemV1.Image = hostReq.ImageID
			return nil
		})
	})
	if xerr != nil {
		return nil, xerr
	}

	// -- Updates host link with subnets --
	//if !hostReq.IsGateway {
	if xerr := rh.updateSubnets(task, hostReq); xerr != nil {
		return nil, xerr
	}
	defer rh.undoUpdateSubnets(task, hostReq, &xerr)
	//}

	if xerr = rh.finalizeProvisioning(task, hostReq, userdataContent); xerr != nil {
		return nil, xerr
	}
	//// Executes userdata.PHASE2_NETWORK_AND_SECURITY script to configure subnet and security
	//if xerr = rh.runInstallPhase(task, userdata.PHASE2_NETWORK_AND_SECURITY, userdataContent); xerr != nil {
	//	return nil, xerr
	//}
	//
	//// Reboot host
	//command := "sudo systemctl reboot"
	//if _, _, _, xerr = rh.Run(task, command, outputs.COLLECT, 0, 0); xerr != nil {
	//	return nil, xerr
	//}
	//
	//// if host is a gateway, executes userdata.PHASE3_GATEWAY_HIGH_AVAILABILITY script to configure subnet and security
	//if !hostReq.IsGateway {
	//	// execute userdata.PHASE4_SYSTEM_FIXES script to fix possible misconfiguration in system
	//	if xerr = rh.runInstallPhase(task, userdata.PHASE4_SYSTEM_FIXES, userdataContent); xerr != nil {
	//		return nil, xerr
	//	}
	//
	//	// Reboot host
	//	command = "sudo systemctl reboot"
	//	if _, _, _, xerr = rh.Run(task, command, outputs.COLLECT, 0, 0); xerr != nil {
	//		return nil, xerr
	//	}
	//
	//	// execute userdata.PHASE5_FINAL script to final install/configure of the host (no need to reboot)
	//	if xerr = rh.runInstallPhase(task, userdata.PHASE5_FINAL, userdataContent); xerr != nil {
	//		return nil, xerr
	//	}
	//
	//	// TODO: configurable timeout here
	//	if status, xerr = rh.waitInstallPhase(task, userdata.PHASE5_FINAL, time.Duration(0)); xerr != nil {
	//		if _, ok := xerr.(*fail.ErrTimeout); ok {
	//			return nil, fail.Wrap(xerr, "timeout creating a host")
	//		}
	//		if abstract.IsProvisioningError(xerr) {
	//			logrus.Errorf("%+v", xerr)
	//			return nil, fail.Wrap(xerr, "error provisioning the new host, please check safescaled logs", rh.GetName())
	//		}
	//		return nil, xerr
	//	}
	//} else {
	//	// TODO: configurable timeout here
	//	if status, xerr = rh.waitInstallPhase(task, userdata.PHASE2_NETWORK_AND_SECURITY, time.Duration(0)); xerr != nil {
	//		if _, ok := xerr.(*fail.ErrTimeout); ok {
	//			return nil, fail.Wrap(xerr, "timeout creating a host")
	//		}
	//		if abstract.IsProvisioningError(xerr) {
	//			logrus.Errorf("%+v", xerr)
	//			return nil, fail.Wrap(xerr, "error provisioning the new host, please check safescaled logs", rh.GetName())
	//		}
	//		return nil, xerr
	//	}
	//}

	logrus.Infof("host '%s' created successfully", rh.GetName())
	return userdataContent, nil
}

// setSecurityGroups sets the Security Groups for the host
func (rh *host) setSecurityGroups(task concurrency.Task, req abstract.HostRequest, defaultSubnet resources.Subnet) fail.Error {
	return rh.properties.Alter(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) (innerXErr fail.Error) {
		hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
		if !ok {
			return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
		}

		var (
			sg resources.SecurityGroup
			as *abstract.Subnet
		)
		svc := rh.GetService()

		subnetCount := len(req.Subnets)
		isolatedHost := !req.IsGateway && req.PublicIP && (subnetCount == 0 || (subnetCount == 1 && defaultSubnet.GetName() == abstract.SingleHostNetworkName))

		//sgs := []string{}
		//if !s.cfgOpts.UseFloatingIP {
		//	if request.IsGateway || (request.PublicIP && (subnetCount == 0 || (subnetCount == 1 && defaultSubnet.Name == abstract.SingleHostNetworkName))) {
		//		sgs = append(sgs, defaultSubnet.GWSecurityGroupID)
		//	}
		//}
		//// DO NOT add Subnet internal Security Group if host is a Single Host with public IP; this kind of host needs to be isolated (not perfect with Security Group but it's a start)
		//if request.IsGateway || !request.PublicIP || (subnetCount == 1 && defaultSubnet.Name != abstract.SingleHostNetworkName) || subnetCount > 1 {
		//	sgs = append(sgs, defaultSubnet.InternalSecurityGroupID)
		//}
		if req.IsGateway || isolatedHost {
			innerXErr = defaultSubnet.Inspect(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
				var ok bool
				as, ok = clonable.(*abstract.Subnet)
				if !ok {
					return fail.InconsistentError("'*abstract.Subnet' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}
				return nil
			})
			if innerXErr != nil {
				return innerXErr
			}

			sg, innerXErr = LoadSecurityGroup(task, svc, as.GWSecurityGroupID)
			if innerXErr != nil {
				return fail.Wrap(innerXErr, "failed to query Subnet '%s' Security Group '%s'", defaultSubnet.GetName(), as.GWSecurityGroupID)
			}
			if innerXErr = sg.BindToHost(task, rh, resources.SecurityGroupEnable, resources.MarkSecurityGroupAsSupplemental); innerXErr != nil {
				return fail.Wrap(innerXErr, "failed to apply Subnet's Security Group for gateway '%s' on host '%s'", sg.GetName(), req.ResourceName)
			}

			defer func() {
				if innerXErr != nil && !req.KeepOnFailure {
					if derr := sg.UnbindFromHost(task, rh); derr != nil {
						_ = innerXErr.AddConsequence(fail.Wrap(derr, "cleaning up on failure, failed to unbind Security Group '%s' from Host '%s'", sg.GetName(), rh.GetName()))
					}
				}
			}()

			item := &propertiesv1.SecurityGroupBond{
				ID:         sg.GetID(),
				Name:       sg.GetName(),
				Disabled:   false,
				FromSubnet: true,
			}
			hsgV1.ByID[item.ID] = item
			hsgV1.ByName[item.Name] = item.ID
		}

		// Apply internal Security Group of each subnets
		if req.IsGateway || !isolatedHost {
			defer func() {
				if innerXErr != nil && !req.KeepOnFailure {
					var (
						derr   error
						errors []error
					)
					for _, v := range req.Subnets {
						if sg, derr = LoadSecurityGroup(task, svc, v.InternalSecurityGroupID); derr == nil {
							derr = sg.UnbindFromHost(task, rh)
						}
						if derr != nil {
							errors = append(errors, derr)
						}
					}
					if len(errors) > 0 {
						_ = innerXErr.AddConsequence(fail.Wrap(fail.NewErrorList(errors), "failed to unbind Subnets Security Group from host '%s'", sg.GetName(), req.ResourceName))
					}
				}
			}()

			for _, v := range req.Subnets {
				sg, xerr := LoadSecurityGroup(task, svc, v.InternalSecurityGroupID)
				if xerr != nil {
					return fail.Wrap(xerr, "failed to load Subnet '%s' internal Security Group %s", v.Name, v.InternalSecurityGroupID)
				}
				if xerr = sg.BindToHost(task, rh, resources.SecurityGroupEnable, resources.MarkSecurityGroupAsSupplemental); xerr != nil {
					return fail.Wrap(xerr, "failed to apply Subnet '%s' internal Security Group '%s' to host '%s'", v.Name, sg.GetName(), req.ResourceName)
				}

				// register security group in properties
				item := &propertiesv1.SecurityGroupBond{
					ID:         sg.GetID(),
					Name:       sg.GetName(),
					Disabled:   false,
					FromSubnet: true,
				}
				hsgV1.ByID[item.ID] = item
				hsgV1.ByName[item.Name] = item.ID
			}
		}

		// Create and bind a dedicated Security Group to the host (with no rules by default)
		var (
			rn resources.Network
			an *abstract.Network
		)
		if rn, innerXErr = defaultSubnet.InspectNetwork(task); innerXErr == nil {
			innerXErr = rn.Inspect(task, func(clonable data.Clonable, _ *serialize.JSONProperties) fail.Error {
				var ok bool
				an, ok = clonable.(*abstract.Network)
				if !ok {
					return fail.InconsistentError("'*abstract.Network' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}
				return nil
			})
		}
		if innerXErr != nil {
			return fail.Wrap(innerXErr, "failed to query Network of Subnet '%s'", defaultSubnet.GetName())
		}
		sgName := fmt.Sprintf(defaultHostSecurityGroupNamePattern, req.ResourceName, defaultSubnet.GetName(), an.Name)
		hostSG, xerr := NewSecurityGroup(svc)
		if xerr != nil {
			return fail.Wrap(xerr, "failed to instantiate a new Security Group")
		}
		if xerr = hostSG.Create(task, an.ID, sgName, fmt.Sprintf("Host %s default Security Group", req.ResourceName), abstract.SecurityGroupRules{}); xerr != nil {
			return fail.Wrap(xerr, "failed to create Host '%s' default Security Group '%s'", req.ResourceName, sgName)
		}

		// Starting from here, delete host Security group if exiting with error
		defer func() {
			if innerXErr != nil && !req.KeepOnFailure {
				if derr := hostSG.Delete(task); derr != nil {
					_ = innerXErr.AddConsequence(fail.Wrap(derr, "cleaning unp on failure, failed to delete Host's Security Group '%s'", hostSG.GetName()))
				}
			}
		}()

		// Bind freshly created Security Group to the host as default
		if xerr = hostSG.BindToHost(task, rh, resources.SecurityGroupEnable, resources.MarkSecurityGroupAsDefault); xerr != nil {
			return fail.Wrap(xerr, "failed to bind Security Group '%s' to host '%s'", sgName, req.ResourceName)
		}

		// Starting from here, unbind hosts security group if exiting with error
		defer func() {
			if innerXErr != nil && !req.KeepOnFailure {
				if derr := hostSG.UnbindFromHost(task, rh); derr != nil {
					_ = innerXErr.AddConsequence(fail.Wrap(derr, "cleaning up on failure, failed to unbind Security Group '%s' from Host '%s'", hostSG.GetName(), rh.GetName()))
				}
			}
		}()

		// register the security group in properties
		item := &propertiesv1.SecurityGroupBond{
			ID:         hostSG.GetID(),
			Name:       hostSG.GetName(),
			Disabled:   false,
			FromSubnet: false,
		}
		hsgV1.ByID[item.ID] = item
		hsgV1.ByName[item.Name] = item.ID
		hsgV1.DefaultID = item.ID

		// Unbind "default" Security Group from Host if it is bound
		if sgName = svc.GetDefaultSecurityGroupName(); sgName != "" {
			rsg, innerXErr := svc.InspectSecurityGroup(sgName)
			if innerXErr != nil {
				switch innerXErr.(type) {
				case *fail.ErrNotFound:
				// ignore this error
				default:
					return innerXErr
				}
			} else if innerXErr = svc.UnbindSecurityGroupFromHost(rsg.GetID(), rh.GetID()); innerXErr != nil {
				switch innerXErr.(type) {
				case *fail.ErrNotFound:
					// Consider a security group not found as a successful unbind
				default:
					return fail.Wrap(innerXErr, "failed to unbind Security Group '%s' from Host", sgName)
				}
			}
		}

		return nil
	})
}

func (rh *host) onFailureUndoSetSecurityGroups(task concurrency.Task, errorPtr *fail.Error, keepOnFailure bool) {
	if errorPtr == nil {
		logrus.Errorf("trying to call a cancel function from a nil error; cancel not run")
		return
	}
	if *errorPtr != nil && !keepOnFailure {
		svc := rh.GetService()
		derr := rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
			return props.Alter(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) (innerXErr fail.Error) {
				hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
				if !ok {
					return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}
				var (
					opXErr fail.Error
					sg     resources.SecurityGroup
					errors []error
				)

				// unbind security groups
				for _, v := range hsgV1.ByName {
					if sg, opXErr = LoadSecurityGroup(task, svc, v); opXErr == nil {
						opXErr = sg.UnbindFromHost(task, rh)
					}
					if opXErr != nil {
						errors = append(errors, opXErr)
					}
				}
				if len(errors) > 0 {
					return fail.Wrap(fail.NewErrorList(errors), "cleaning up on failure, failed to unbind Security Groups from Host")
				}

				// delete host default security group
				if hsgV1.DefaultID != "" {
					sg, innerXErr = LoadSecurityGroup(task, svc, hsgV1.DefaultID)
					if innerXErr != nil {
						switch innerXErr.(type) {
						case *fail.ErrNotFound:
						// consider non existence as a deletion success
						default:
							return innerXErr
						}
					} else {
						if innerXErr = sg.Delete(task); innerXErr != nil {
							return innerXErr
						}
					}
				}
				return nil
			})
		})
		if derr != nil {
			_ = (*errorPtr).AddConsequence(fail.Wrap(derr, "cleaning up on failure, failed to cleanup Security Groups"))
		}
	}
}

func (rh host) findTemplateID(hostDef abstract.HostSizingRequirements) (string, fail.Error) {
	svc := rh.GetService()
	useScannerDB := hostDef.MinGPU > 0 || hostDef.MinCPUFreq > 0
	templates, xerr := svc.SelectTemplatesBySize(hostDef, useScannerDB)
	if xerr != nil {
		return "", fail.Wrap(xerr, "failed to find template corresponding to requested resources")
	}
	var template abstract.HostTemplate
	if len(templates) > 0 {
		template = *(templates[0])
		msg := fmt.Sprintf("Selected host template: '%s' (%d core%s", template.Name, template.Cores, strprocess.Plural(uint(template.Cores)))
		if template.CPUFreq > 0 {
			msg += fmt.Sprintf(" at %.01f GHz", template.CPUFreq)
		}
		msg += fmt.Sprintf(", %.01f GB RAM, %d GB disk", template.RAMSize, template.DiskSize)
		if template.GPUNumber > 0 {
			msg += fmt.Sprintf(", %d GPU%s", template.GPUNumber, strprocess.Plural(uint(template.GPUNumber)))
			if template.GPUType != "" {
				msg += fmt.Sprintf(" %s", template.GPUType)
			}
		}
		msg += ")"
		logrus.Infof(msg)
	} else {
		logrus.Errorf("failed to find template corresponding to requested resources")
		return "", fail.Wrap(xerr, "failed to find template corresponding to requested resources")
	}
	return template.ID, nil
}

func (rh host) findImageID(hostDef *abstract.HostSizingRequirements) (string, fail.Error) {
	svc := rh.GetService()
	if hostDef.Image == "" {
		cfg, xerr := svc.GetConfigurationOptions()
		if xerr != nil {
			return "", xerr
		}
		hostDef.Image = cfg.GetString("DefaultImage")
	}

	var img *abstract.Image
	xerr := retry.WhileUnsuccessfulDelay1Second(
		func() error {
			var innerXErr fail.Error
			img, innerXErr = svc.SearchImage(hostDef.Image)
			return innerXErr
		},
		10*time.Second,
	)
	if xerr != nil {
		return "", xerr
	}
	return img.ID, nil
}

// runInstallPhase uploads then starts script corresponding to phase 'phase'
func (rh host) runInstallPhase(task concurrency.Task, phase userdata.Phase, userdataContent *userdata.Content) fail.Error {
	// execute userdata 'final' (phase4) script to final install/configure of the host (no need to reboot)
	content, xerr := userdataContent.Generate(phase)
	if xerr != nil {
		return xerr
	}
	file := fmt.Sprintf("/opt/safescale/var/tmp/user_data.%s.sh", phase)
	if xerr = rh.PushStringToFile(task, string(content), file, "", ""); xerr != nil {
		return xerr
	}
	command := fmt.Sprintf("sudo bash %s; exit $?", file)
	// Executes the script on the remote host
	retcode, _, stderr, xerr := rh.Run(task, command, outputs.COLLECT, 0, 0)
	if xerr != nil {
		return fail.Wrap(xerr, "failed to apply configuration phase '%s'", phase)
	}
	if retcode != 0 {
		if retcode == 255 {
			return fail.NewError("failed to execute install phase '%s' on host '%s': SSH connection failed", phase, rh.GetName())
		}
		return fail.NewError("failed to execute install phase '%s' on host '%s': %s", phase, rh.GetName(), stderr)
	}
	return nil
}

func (rh *host) waitInstallPhase(task concurrency.Task, phase userdata.Phase, timeout time.Duration) (string, fail.Error) {
	sshDefaultTimeout := int(temporal.GetHostTimeout().Minutes())
	if sshDefaultTimeoutCandidate := os.Getenv("SSH_TIMEOUT"); sshDefaultTimeoutCandidate != "" {
		num, err := strconv.Atoi(sshDefaultTimeoutCandidate)
		if err == nil {
			logrus.Debugf("Using custom timeout of %d minutes", num)
			sshDefaultTimeout = num
		}
	}
	sshCfg := rh.getSSHConfig(task)

	// TODO: configurable timeout here
	status, xerr := sshCfg.WaitServerReady(task, string(phase), time.Duration(sshDefaultTimeout)*time.Minute)
	if xerr != nil {
		if _, ok := xerr.(*fail.ErrTimeout); ok {
			return status, fail.Wrap(xerr, "timeout creating a host")
		}
		if abstract.IsProvisioningError(xerr) {
			logrus.Errorf("%+v", xerr)
			return status, fail.Wrap(xerr, "error creating host '%s': error provisioning the new host, please check safescaled logs", rh.GetName())
		}
		return status, xerr
	}
	return status, nil
}

// updateSubnets updates subnets on which host is attached and host property HostNetworkV2
func (rh *host) updateSubnets(task concurrency.Task, req abstract.HostRequest) fail.Error {
	// If host is a gateway, do not add it as host attached to the Subnet, it's considered as part of the subnet
	if !req.IsGateway {
		return rh.Alter(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
			return props.Alter(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
				hnV2, ok := clonable.(*propertiesv2.HostNetworking)
				if !ok {
					return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}

				hostID := rh.GetID()
				hostName := rh.GetName()

				for _, as := range req.Subnets {
					rs, innerXErr := LoadSubnet(task, rh.core.GetService(), "", as.ID)
					if innerXErr != nil {
						return innerXErr
					}

					innerXErr = rs.Alter(task, func(clonable data.Clonable, properties *serialize.JSONProperties) fail.Error {
						return properties.Alter(task, subnetproperty.HostsV1, func(clonable data.Clonable) fail.Error {
							subnetHostsV1, ok := clonable.(*propertiesv1.SubnetHosts)
							if !ok {
								return fail.InconsistentError("'*propertiesv1.SubnetHosts' expected, '%s' provided", reflect.TypeOf(clonable).String())
							}

							subnetHostsV1.ByName[hostName] = hostID
							subnetHostsV1.ByID[hostID] = hostName
							return nil
						})
					})
					if innerXErr != nil {
						return innerXErr
					}

					hnV2.SubnetsByID[as.ID] = as.Name
					hnV2.SubnetsByName[as.Name] = as.ID
				}
				return nil
			})
		})
	}
	return nil
}

// undoUpdateSubnets removes what updateSubnets have done
func (rh *host) undoUpdateSubnets(task concurrency.Task, req abstract.HostRequest, errorPtr *fail.Error) {
	if errorPtr != nil && *errorPtr != nil && !req.IsGateway && !req.KeepOnFailure {
		xerr := rh.Alter(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
			return props.Alter(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
				hsV1, ok := clonable.(*propertiesv2.HostNetworking)
				if !ok {
					return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}

				hostID := rh.GetID()
				hostName := rh.GetName()

				for _, as := range req.Subnets {
					rs, innerXErr := LoadSubnet(task, rh.core.GetService(), "", as.ID)
					if innerXErr != nil {
						return innerXErr
					}

					innerXErr = rs.Alter(task, func(clonable data.Clonable, properties *serialize.JSONProperties) fail.Error {
						return properties.Alter(task, subnetproperty.HostsV1, func(clonable data.Clonable) fail.Error {
							subnetHostsV1, ok := clonable.(*propertiesv1.SubnetHosts)
							if !ok {
								return fail.InconsistentError("'*propertiesv1.SubnetHosts' expected, '%s' provided", reflect.TypeOf(clonable).String())
							}

							delete(subnetHostsV1.ByID, hostID)
							delete(subnetHostsV1.ByName, hostName)
							return nil
						})
					})
					if innerXErr != nil {
						return innerXErr
					}

					delete(hsV1.SubnetsByID, as.ID)
					delete(hsV1.SubnetsByName, as.ID)
				}
				return nil
			})
		})
		if xerr != nil {
			_ = (*errorPtr).AddConsequence(fail.Wrap(xerr, "cleaning up on failure, failed to remove Host relationships with Subnets"))
		}
	}
}

func (rh *host) finalizeProvisioning(task concurrency.Task, req abstract.HostRequest, userdataContent *userdata.Content) fail.Error {
	// Executes userdata.PHASE2_NETWORK_AND_SECURITY script to configure subnet and security
	if xerr := rh.runInstallPhase(task, userdata.PHASE2_NETWORK_AND_SECURITY, userdataContent); xerr != nil {
		return xerr
	}

	// Reboot host
	command := "sudo systemctl reboot"
	if _, _, _, xerr := rh.Run(task, command, outputs.COLLECT, 0, 0); xerr != nil {
		return xerr
	}

	// if host is not a gateway, executes userdata.PHASE4 and 5 script to configure subnet and security
	// For a gateway, userdata.PHASE3 to 5 have to be run explicitly (cf. operations/subnet.go)
	if !req.IsGateway {
		// execute userdata.PHASE4_SYSTEM_FIXES script to fix possible misconfiguration in system
		if xerr := rh.runInstallPhase(task, userdata.PHASE4_SYSTEM_FIXES, userdataContent); xerr != nil {
			return xerr
		}

		// Reboot host
		command = "sudo systemctl reboot"
		if _, _, _, xerr := rh.Run(task, command, outputs.COLLECT, 0, 0); xerr != nil {
			return xerr
		}

		// execute userdata.PHASE5_FINAL script to final install/configure of the host (no need to reboot)
		if xerr := rh.runInstallPhase(task, userdata.PHASE5_FINAL, userdataContent); xerr != nil {
			return xerr
		}

		// TODO: configurable timeout here
		if _, xerr := rh.waitInstallPhase(task, userdata.PHASE5_FINAL, time.Duration(0)); xerr != nil {
			if _, ok := xerr.(*fail.ErrTimeout); ok {
				return fail.Wrap(xerr, "timeout creating a host")
			}
			if abstract.IsProvisioningError(xerr) {
				logrus.Errorf("%+v", xerr)
				return fail.Wrap(xerr, "error provisioning the new host, please check safescaled logs", rh.GetName())
			}
			return xerr
		}
	}
	return nil
}

// WaitSSHReady waits until SSH responds successfully
func (rh *host) WaitSSHReady(task concurrency.Task, timeout time.Duration) (string, fail.Error) {
	if rh.IsNull() {
		return "", fail.InvalidInstanceError()
	}
	if task.IsNull() {
		return "", fail.InvalidParameterError("task", "cannot be nil")
	}

	return rh.waitInstallPhase(task, userdata.PHASE5_FINAL, timeout)
}

// getOrCreateDefaultSubnet gets network abstract.SingleHostNetworkName or create it if necessary
func getOrCreateDefaultSubnet(task concurrency.Task, svc iaas.Service) (rs resources.Subnet, gw resources.Host, xerr fail.Error) {
	rn, xerr := LoadNetwork(task, svc, abstract.SingleHostNetworkName)
	if xerr != nil {
		switch xerr.(type) {
		case *fail.ErrNotFound:
		// continue
		default:
			return nil, nil, xerr
		}
	}
	if rn == nil {
		rn, xerr = NewNetwork(svc)
		if xerr != nil {
			return nil, nil, xerr
		}
		req := abstract.NetworkRequest{
			Name: abstract.SingleHostNetworkName,
			CIDR: stacks.DefaultNetworkCIDR,
		}
		xerr = rn.Create(task, req)
		if xerr != nil {
			return nil, nil, xerr
		}

		defer func() {
			if xerr != nil {
				derr := rn.Delete(task)
				if derr != nil {
					_ = xerr.AddConsequence(derr)
				}
			}
		}()
	}

	rs, xerr = LoadSubnet(task, svc, rn.GetID(), abstract.SingleHostSubnetName)
	if xerr != nil {
		switch xerr.(type) {
		case *fail.ErrNotFound:
		default:
			return nil, nil, xerr
		}
	}
	if rs.IsNull() {
		rs, xerr = NewSubnet(svc)
		if xerr != nil {
			return nil, nil, xerr
		}
		var DNSServers []string
		if opts, xerr := svc.GetConfigurationOptions(); xerr != nil {
			switch xerr.(type) {
			case *fail.ErrNotFound:
			default:
				return nil, nil, xerr
			}
		} else {
			DNSServers = strings.Split(opts.GetString("DNSServers"), ",")
		}
		req := abstract.SubnetRequest{
			Name:       abstract.SingleHostSubnetName,
			CIDR:       "10.0.0.0/17",
			DNSServers: DNSServers,
			HA:         false,
		}
		if xerr = rs.Create(task, req, "", nil); xerr != nil {
			return nil, nil, xerr
		}

		defer func() {
			if xerr != nil {
				derr := rs.Delete(task)
				if derr != nil {
					_ = xerr.AddConsequence(fail.Wrap(derr, "cleaning up on failure, failed to delete subnet '%s'", abstract.SingleHostSubnetName))
				}
			}
		}()
	}

	rh, xerr := rs.GetGateway(task, true)
	if xerr != nil {
		return nil, nil, xerr
	}

	return rs, rh, nil
}

// Delete deletes a host with its metadata and updates subnet links
func (rh *host) Delete(task concurrency.Task) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task.IsNull() {
		return fail.InvalidParameterError("task", "cannot be nil")
	}

	rh.SafeLock(task)
	defer rh.SafeUnlock(task)

	xerr := rh.Alter(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
		// Don't remove a host that is a gateway
		return props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hostNetworkV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			if hostNetworkV2.IsGateway {
				return fail.NotAvailableError("cannot delete host, it's a gateway that can only be deleted through its subnet")
			}
			return nil
		})

		//// Don't remove a host having shares that are currently remotely mounted
		//var shares map[string]*propertiesv1.HostShare
		//innerXErr = properties.Inspect(task, hostproperty.SharesV1, func(clonable data.Clonable) fail.Error {
		//	sharesV1, ok := clonable.(*propertiesv1.HostShares)
		//	if !ok {
		//		return fail.InconsistentError("'*propertiesv1.HostShares' expected, '%s' provided", reflect.TypeOf(clonable).String())
		//	}
		//	shares = sharesV1.ByID
		//	shareCount := len(shares)
		//	for _, hostShare := range shares {
		//		count := len(hostShare.ClientsByID)
		//		if count > 0 {
		//			// clients found, checks if these clients already exists...
		//			for _, hostID := range hostShare.ClientsByID {
		//				_, inErr := LoadHost(task, svc, hostID)
		//				if inErr == nil {
		//					return fail.NotAvailableError("exports %d share%s and at least one share is mounted", shareCount, strprocess.Plural(uint(shareCount)))
		//				}
		//			}
		//		}
		//	}
		//	return nil
		//})
		//if innerXErr != nil {
		//	return innerXErr
		//}

		//// Don't remove a host with volumes attached
		//innerXErr = properties.Inspect(task, hostproperty.VolumesV1, func(clonable data.Clonable) fail.Error {
		//	hostVolumesV1, ok := clonable.(*propertiesv1.HostVolumes)
		//	if !ok {
		//		return fail.InconsistentError("'*propertiesv1.HostVolumes' expected, '%s' provided", reflect.TypeOf(clonable).String())
		//	}
		//	nAttached := len(hostVolumesV1.VolumesByID)
		//	if nAttached > 0 {
		//		return fail.NotAvailableError("host has %d volume%s attached", nAttached, strprocess.Plural(uint(nAttached)))
		//	}
		//	return nil
		//})
		//if innerXErr != nil {
		//	return innerXErr
		//}
	})
	if xerr != nil {
		return xerr
	}

	return rh.relaxedDeleteHost(task)

	//	// If host mounted shares, unmounts them before anything else
	//	var mounts []*propertiesv1.HostShare
	//	innerXErr = properties.Inspect(task, hostproperty.MountsV1, func(clonable data.Clonable) fail.Error {
	//		hostMountsV1, ok := clonable.(*propertiesv1.HostMounts)
	//		if !ok {
	//			return fail.InconsistentError("'*propertiesv1.HostMounts' expected, '%s' provided", reflect.TypeOf(clonable).String())
	//		}
	//		for _, i := range hostMountsV1.RemoteMountsByPath {
	//			// Retrieve share data
	//			objs, loopErr := NewShare(svc)
	//			if loopErr != nil {
	//				return loopErr
	//			}
	//			loopErr = objs.Read(task, i.ShareID)
	//			if loopErr != nil {
	//				return loopErr
	//			}
	//
	//			// Retrieve data about the server serving the share
	//			objserver, loopErr := objs.GetServer(task)
	//			if loopErr != nil {
	//				return loopErr
	//			}
	//			// Retrieve data about share from its server
	//			share, loopErr := objserver.GetShare(task, i.ShareID)
	//			if loopErr != nil {
	//				return loopErr
	//			}
	//			mounts = append(mounts, share)
	//		}
	//		return nil
	//	})
	//	if innerXErr != nil {
	//		return innerXErr
	//	}
	//
	//	// Unmounts tier shares mounted on host (done outside the previous host.properties.Reading() section, because
	//	// Unmount() have to lock for write, and won't succeed while host.properties.Reading() is running,
	//	// leading to a deadlock)
	//	for _, item := range mounts {
	//		objs, loopErr := LoadShare(task, svc, item.ID)
	//		if loopErr != nil {
	//			return loopErr
	//		}
	//		loopErr = objs.Unmount(task, rh)
	//		if loopErr != nil {
	//			return loopErr
	//		}
	//	}
	//
	//	// if host exports shares, delete them
	//	for _, share := range shares {
	//		objs, loopErr := LoadShare(task, svc, share.Name)
	//		if loopErr != nil {
	//			return loopErr
	//		}
	//		loopErr = objs.Delete(task)
	//		if loopErr != nil {
	//			return loopErr
	//		}
	//	}
	//
	//	// Update networks property propertiesv1.HostNetworking to remove the reference to the host
	//	innerXErr = properties.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
	//		hostNetworkV2, ok := clonable.(*propertiesv2.HostNetworking)
	//		if !ok {
	//			return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
	//		}
	//		hostID := rh.GetID()
	//		hostName := rh.GetName()
	//		var errors []error
	//		for k := range hostNetworkV2.SubnetsByID {
	//			rn, loopErr := LoadNetwork(task, svc, k)
	//			if loopErr != nil {
	//				logrus.Errorf(loopErr.Error())
	//				errors = append(errors, loopErr)
	//				continue
	//			}
	//			loopErr = rn.Alter(task, func(_ data.Clonable, netprops *serialize.JSONProperties) fail.Error {
	//				return netprops.Alter(task, networkproperty.HostsV1, func(clonable data.Clonable) fail.Error {
	//					networkHostsV1, ok := clonable.(*propertiesv1.NetworkHosts)
	//					if !ok {
	//						return fail.InconsistentError("'*propertiesv1.NetworkHosts' expected, '%s' provided", reflect.TypeOf(clonable).String())
	//					}
	//					delete(networkHostsV1.ByID, hostID)
	//					delete(networkHostsV1.ByName, hostName)
	//					return nil
	//				})
	//			})
	//			if loopErr != nil {
	//				logrus.Errorf(loopErr.Error())
	//				errors = append(errors, loopErr)
	//			}
	//		}
	//		return fail.NewErrorList(errors)
	//	})
	//	if innerXErr != nil {
	//		return innerXErr
	//	}
	//
	//	// Unbind Security Group from host
	//	innerXErr = properties.Alter(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) fail.Error {
	//		hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
	//		if !ok {
	//			return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
	//		}
	//
	//		// Unbind Security Groups from Host
	//		var errors []error
	//		for _, v := range hsgV1.ByID {
	//			rsg, derr := LoadSecurityGroup(task, svc, v.ID)
	//			if derr == nil {
	//				derr = rsg.UnbindFromHost(task, rh)
	//			}
	//			if derr != nil {
	//				switch derr.(type) {
	//				case *fail.ErrNotFound:
	//					// Consider that a Security Group that cannot be loaded or is not bound as a success
	//				default:
	//					errors = append(errors, derr)
	//				}
	//			}
	//		}
	//		if len(errors) > 0 {
	//			return fail.Wrap(fail.NewErrorList(errors), "failed to unbind some Security Groups")
	//		}
	//
	//		// Delete default Security Group of Host
	//		if hsgV1.DefaultID != "" {
	//			rsg, derr := LoadSecurityGroup(task, svc, hsgV1.DefaultID)
	//			if derr == nil {
	//				derr = rsg.Delete(task)
	//			}
	//			if derr != nil {
	//				switch derr.(type) {
	//				case *fail.ErrNotFound:
	//					// Consider a Security Group that cannot be found as a success
	//				default:
	//					return fail.Wrap(derr, "failed to delete default Security Group of Host")
	//				}
	//			}
	//		}
	//		return nil
	//	})
	//	if innerXErr != nil {
	//		return fail.Wrap(innerXErr, "failed to unbind Security Groups from Host")
	//	}
	//
	//	// Conditions are met, delete host
	//	waitForDeletion := true
	//	delErr := retry.WhileUnsuccessfulDelay1Second(
	//		func() error {
	//			// FIXME: need to remove retry from svc.DeleteHost!
	//			err := svc.DeleteHost(hostID)
	//			if err != nil {
	//				if _, ok := err.(*fail.ErrNotFound); !ok {
	//					return fail.Wrap(err, "cannot delete host")
	//				}
	//				// logrus.Warn("host resource not found on provider side, host metadata will be removed for consistency")
	//				waitForDeletion = false
	//			}
	//			return nil
	//		},
	//		time.Minute*5,
	//	)
	//	if delErr != nil {
	//		return delErr
	//	}
	//
	//	// wait for effective host deletion
	//	if waitForDeletion {
	//		innerXErr = retry.WhileUnsuccessfulDelay5SecondsTimeout(
	//			func() error {
	//				// FIXME: need to remove retry from svc.GetHostState if the issues are not communication issues!
	//				if state, stateErr := svc.GetHostState(rh.GetID()); stateErr == nil {
	//					logrus.Warnf("While deleting the status was [%s]", state)
	//					if state == hoststate.ERROR {
	//						return fail.NotAvailableError("host is in state ERROR")
	//					}
	//				} else {
	//					return stateErr
	//				}
	//				return nil
	//			},
	//			time.Minute*2, // FIXME: static duration
	//		)
	//		if innerXErr != nil {
	//			return innerXErr
	//		}
	//	}
	//
	//	return nil
	//})
	//if xerr != nil {
	//	return xerr
	//}
	//
	//// Deletes metadata from Object Storage
	//if xerr = rh.core.Delete(task); xerr != nil {
	//	// If entry not found, considered as success
	//	if _, ok := xerr.(*fail.ErrNotFound); !ok {
	//		return xerr
	//	}
	//}
	//
	//newHost := nullHost()
	//*rh = *newHost
	//return nil
}

// relaxedDeleteHost is the method that really deletes a host, being a gateway or not
func (rh *host) relaxedDeleteHost(task concurrency.Task) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}

	rh.SafeLock(task)
	defer rh.SafeUnlock(task)

	svc := rh.GetService()

	var shares map[string]*propertiesv1.HostShare
	xerr := rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		// Don't remove a host having shares that are currently remotely mounted
		innerXErr := props.Inspect(task, hostproperty.SharesV1, func(clonable data.Clonable) fail.Error {
			sharesV1, ok := clonable.(*propertiesv1.HostShares)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostShares' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			shares = sharesV1.ByID
			shareCount := len(shares)
			for _, hostShare := range shares {
				count := len(hostShare.ClientsByID)
				if count > 0 {
					// clients found, checks if these clients already exists...
					for _, hostID := range hostShare.ClientsByID {
						_, inErr := LoadHost(task, svc, hostID)
						if inErr == nil {
							return fail.NotAvailableError("exports %d share%s and at least one share is mounted", shareCount, strprocess.Plural(uint(shareCount)))
						}
					}
				}
			}
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}

		// Don't remove a host with volumes attached
		return props.Inspect(task, hostproperty.VolumesV1, func(clonable data.Clonable) fail.Error {
			hostVolumesV1, ok := clonable.(*propertiesv1.HostVolumes)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostVolumes' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			nAttached := len(hostVolumesV1.VolumesByID)
			if nAttached > 0 {
				return fail.NotAvailableError("host has %d volume%s attached", nAttached, strprocess.Plural(uint(nAttached)))
			}
			return nil
		})
	})
	if xerr != nil {
		return xerr
	}

	xerr = rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		// If host mounted shares, unmounts them before anything else
		var mounts []*propertiesv1.HostShare
		innerXErr := props.Inspect(task, hostproperty.MountsV1, func(clonable data.Clonable) fail.Error {
			hostMountsV1, ok := clonable.(*propertiesv1.HostMounts)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostMounts' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			for _, i := range hostMountsV1.RemoteMountsByPath {
				// Retrieve item data
				rshare, loopErr := LoadShare(task, svc, i.ShareID)
				if loopErr != nil {
					return loopErr
				}

				// Retrieve data about the server serving the item
				rhServer, loopErr := rshare.GetServer(task)
				if loopErr != nil {
					return loopErr
				}
				// Retrieve data about item from its server
				item, loopErr := rhServer.GetShare(task, i.ShareID)
				if loopErr != nil {
					return loopErr
				}
				mounts = append(mounts, item)
			}
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}

		// Unmounts tier shares mounted on host (done outside the previous host.properties.Reading() section, because
		// Unmount() have to lock for write, and won't succeed while host.properties.Reading() is running,
		// leading to a deadlock)
		for _, item := range mounts {
			objs, loopErr := LoadShare(task, svc, item.ID)
			if loopErr != nil {
				return loopErr
			}
			loopErr = objs.Unmount(task, rh)
			if loopErr != nil {
				return loopErr
			}
		}

		// if host exports shares, delete them
		for _, v := range shares {
			objs, loopErr := LoadShare(task, svc, v.Name)
			if loopErr != nil {
				return loopErr
			}
			loopErr = objs.Delete(task)
			if loopErr != nil {
				return loopErr
			}
		}

		// Update networks property propertiesv1.HostNetworking to remove the reference to the host
		innerXErr = props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hostNetworkV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			hostID := rh.GetID()
			hostName := rh.GetName()
			var errors []error
			for k := range hostNetworkV2.SubnetsByID {
				rs, loopErr := LoadSubnet(task, svc, "", k)
				if loopErr != nil {
					logrus.Errorf(loopErr.Error())
					errors = append(errors, loopErr)
					continue
				}
				loopErr = rs.Alter(task, func(_ data.Clonable, netprops *serialize.JSONProperties) fail.Error {
					return netprops.Alter(task, subnetproperty.HostsV1, func(clonable data.Clonable) fail.Error {
						subnetHostsV1, ok := clonable.(*propertiesv1.SubnetHosts)
						if !ok {
							return fail.InconsistentError("'*propertiesv1.SubnetHosts' expected, '%s' provided", reflect.TypeOf(clonable).String())
						}
						delete(subnetHostsV1.ByID, hostID)
						delete(subnetHostsV1.ByName, hostName)
						return nil
					})
				})
				if loopErr != nil {
					logrus.Errorf(loopErr.Error())
					errors = append(errors, loopErr)
				}
			}
			if len(errors) > 0 {
				return fail.Wrap(fail.NewErrorList(errors), "failed to update metadata for Subnets of Host")
			}
			return nil
		})
		if innerXErr != nil {
			return innerXErr
		}

		// Unbind Security Group from host
		innerXErr = props.Alter(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) fail.Error {
			hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}

			// Unbind Security Groups from Host
			var errors []error
			for _, v := range hsgV1.ByID {
				rsg, derr := LoadSecurityGroup(task, svc, v.ID)
				if derr == nil {
					derr = rsg.UnbindFromHost(task, rh)
				}
				if derr != nil {
					switch derr.(type) {
					case *fail.ErrNotFound:
						// Consider that a Security Group that cannot be loaded or is not bound as a success
					default:
						errors = append(errors, derr)
					}
				}
			}
			if len(errors) > 0 {
				return fail.Wrap(fail.NewErrorList(errors), "failed to unbind some Security Groups")
			}

			// Delete default Security Group of Host
			if hsgV1.DefaultID != "" {
				rsg, derr := LoadSecurityGroup(task, svc, hsgV1.DefaultID)
				if derr == nil {
					derr = rsg.Delete(task)
				}
				if derr != nil {
					switch derr.(type) {
					case *fail.ErrNotFound:
						// Consider a Security Group that cannot be found as a success
					default:
						return fail.Wrap(derr, "failed to delete default Security Group of Host")
					}
				}
			}
			return nil
		})
		if innerXErr != nil {
			return fail.Wrap(innerXErr, "failed to unbind Security Groups from Host")
		}

		// Conditions are met, delete host
		waitForDeletion := true
		innerXErr = retry.WhileUnsuccessfulDelay1Second(
			func() error {
				derr := svc.DeleteHost(rh.GetID())
				if derr != nil {
					switch derr.(type) {
					case *fail.ErrNotFound:
						// A host not found is considered as a successful deletion
					default:
						return fail.Wrap(derr, "cannot delete host")
					}
					waitForDeletion = false
				}
				return nil
			},
			time.Minute*5, // FIXME: hardcoded timeout
		)
		if innerXErr != nil {
			return innerXErr
		}

		// wait for effective host deletion
		if waitForDeletion {
			innerXErr = retry.WhileUnsuccessfulDelay5SecondsTimeout(
				func() error {
					state, stateErr := svc.GetHostState(rh.GetID())
					if stateErr != nil {
						switch stateErr.(type) {
						case *fail.ErrNotFound:
						// If host is not found anymore, consider this as a success
						default:
							return stateErr
						}
					}
					logrus.Warnf("Waiting for host deletion, host status is '%s'", state)
					if state == hoststate.ERROR {
						return fail.NotAvailableError("host is in state ERROR")
					}
					return nil
				},
				time.Minute*2, // FIXME: hardcoded duration
			)
			if innerXErr != nil {
				switch innerXErr.(type) {
				case *retry.ErrStopRetry:
					innerXErr = fail.ToError(innerXErr.Cause())
				}
			}
			if innerXErr != nil {
				switch innerXErr.(type) {
				case *fail.ErrNotFound:
				// continue
				default:
					return innerXErr
				}
			}
		}

		return nil
	})
	if xerr != nil {
		return xerr
	}

	// Deletes metadata from Object Storage
	if xerr = rh.core.Delete(task); xerr != nil {
		// If entry not found, considered as success
		if _, ok := xerr.(*fail.ErrNotFound); !ok {
			return xerr
		}
	}

	newHost := nullHost()
	*rh = *newHost
	return nil
}

// GetSSHConfig loads SSH configuration for host from metadata
//
// FIXME: system.SSHConfig should be able to carry data about secondary getGateway
//        Currently, if primary gateway is down, ssh to a host in the subnet will fail
func (rh host) GetSSHConfig(task concurrency.Task) (*system.SSHConfig, fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task.IsNull() {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}

	return rh.getSSHConfig(task), nil
}

// getSSHConfig loads SSH configuration for host from metadata, for internal use
func (rh host) getSSHConfig(task concurrency.Task) *system.SSHConfig {
	if rh.IsNull() || task == nil {
		return &system.SSHConfig{}
	}

	rh.SafeRLock(task)
	defer rh.SafeRUnlock(task)
	return rh.sshProfile
}

// Run tries to execute command 'cmd' on the host
func (rh host) Run(task concurrency.Task, cmd string, outs outputs.Enum, connectionTimeout, executionTimeout time.Duration) (int, string, string, fail.Error) {
	if rh.IsNull() {
		return 0, "", "", fail.InvalidInstanceError()
	}
	if task == nil {
		return 0, "", "", fail.InvalidParameterError("task", "cannot be nil")
	}
	if cmd == "" {
		return 0, "", "", fail.InvalidParameterError("cmd", "cannot be empty string")
	}

	var (
		stdOut, stdErr string
		retCode        int
	)

	// retrieve ssh config to perform some commands
	ssh, xerr := rh.GetSSHConfig(task)
	if xerr != nil {
		return 0, "", "", xerr
	}

	if executionTimeout < temporal.GetHostTimeout() {
		executionTimeout = temporal.GetHostTimeout()
	}
	if connectionTimeout < temporal.DefaultConnectionTimeout {
		connectionTimeout = temporal.DefaultConnectionTimeout
	}
	if connectionTimeout > executionTimeout {
		connectionTimeout = executionTimeout + temporal.GetContextTimeout()
	}

	hostName := rh.GetName()
	xerr = retry.WhileUnsuccessfulDelay1SecondWithNotify(
		func() error {
			var innerXErr fail.Error
			retCode, stdOut, stdErr, innerXErr = run(task, ssh, cmd, outs, executionTimeout)
			if _, ok := innerXErr.(*fail.ErrTimeout); ok {
				innerXErr = fail.NewError("failed to run command in %v delay", executionTimeout)
			}
			return innerXErr
		},
		connectionTimeout,
		func(t retry.Try, v verdict.Enum) {
			if v == verdict.Retry {
				logrus.Printf("Remote SSH service on host '%s' isn't ready, retrying...", hostName)
			}
		},
	)
	return retCode, stdOut, stdErr, xerr
}

// run executes command on the host
func run(task concurrency.Task, ssh *system.SSHConfig, cmd string, outs outputs.Enum, timeout time.Duration) (int, string, string, fail.Error) {
	// Create the command
	sshCmd, xerr := ssh.Command(task, cmd)
	if xerr != nil {
		return 0, "", "", xerr
	}

	retcode, stdout, stderr, xerr := sshCmd.RunWithTimeout(task, outs, timeout)
	if xerr != nil {
		if _, ok := xerr.(*fail.ErrExecution); ok {
			// Adds stdout annotation to xerr
			_ = xerr.Annotate("stdout", stdout)
		}
		return -1, "", "", xerr
	}
	// If retcode == 255, ssh connection failed
	if retcode == 255 {
		return -1, "", "", fail.NewError("failed to connect")
	}
	return retcode, stdout, stderr, xerr
}

// Pull downloads a file from host
func (rh host) Pull(task concurrency.Task, target, source string, timeout time.Duration) (int, string, string, fail.Error) {
	if rh.IsNull() {
		return 0, "", "", fail.InvalidInstanceError()
	}
	if source == "" {
		return 0, "", "", fail.InvalidParameterError("source", "cannot be empty string")
	}
	if target == "" {
		return 0, "", "", fail.InvalidParameterError("target", "cannot be empty string")
	}

	// retrieve ssh config to perform some commands
	ssh, xerr := rh.GetSSHConfig(task)
	if xerr != nil {
		return 0, "", "", xerr
	}

	// FIXME: reintroduce timeout on ssh.
	// if timeout < temporal.GetHostTimeout() {
	// 	timeout = temporal.GetHostTimeout()
	// }
	return ssh.Copy(task, target, source, false)
}

// Push uploads a file to host
func (rh host) Push(task concurrency.Task, source, target, owner, mode string, timeout time.Duration) (int, string, string, fail.Error) {
	if rh.IsNull() {
		return 0, "", "", fail.InvalidInstanceError()
	}
	if source == "" {
		return 0, "", "", fail.InvalidParameterError("source", "cannot be empty string")
	}
	if target == "" {
		return 0, "", "", fail.InvalidParameterError("target", "cannot be empty string")
	}

	// retrieve ssh config to perform some commands
	ssh, xerr := rh.GetSSHConfig(task)
	if xerr != nil {
		return 0, "", "", xerr
	}

	if timeout < temporal.GetHostTimeout() {
		timeout = temporal.GetHostTimeout()
	}

	retcode, stdout, stderr, xerr := ssh.Copy(task, target, source, true)
	if xerr != nil {
		return retcode, stdout, stderr, xerr
	}

	cmd := ""
	if owner != "" {
		cmd += "chown " + owner + ` '` + target + `' ;`
	}
	if mode != "" {
		cmd += "chmod " + mode + ` '` + target + `'`
	}
	if cmd != "" {
		retcode, stdout, stderr, xerr = run(task, ssh, cmd, outputs.DISPLAY, timeout)
	}
	return retcode, stdout, stderr, xerr
}

// GetShare returns a clone of the propertiesv1.HostShare corresponding to share 'shareRef'
func (rh host) GetShare(task concurrency.Task, shareRef string) (*propertiesv1.HostShare, fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task == nil {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}
	if shareRef == "" {
		return nil, fail.InvalidParameterError("shareRef", "cannot be empty string")
	}

	var (
		hostShare *propertiesv1.HostShare
		// ok        bool
	)
	err := rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		// rh, ok := clonable.(*abstract.Host)
		// if !ok {
		// 	return fail.InconsistentError("'*abstract.Host' expected, '%s' provided", reflect.TypeOf(clonable).String()
		// }
		// props, inErr := rh.properties(task)
		// if inErr != nil {
		// 	return inErr
		// }
		return props.Inspect(task, hostproperty.SharesV1, func(clonable data.Clonable) fail.Error {
			sharesV1, ok := clonable.(*propertiesv1.HostShares)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostShares' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			if hostShare, ok = sharesV1.ByID[shareRef].Clone().(*propertiesv1.HostShare); ok {
				return nil
			}
			if _, ok := sharesV1.ByName[shareRef]; ok {
				hostShare = sharesV1.ByID[sharesV1.ByName[shareRef]].Clone().(*propertiesv1.HostShare)
				return nil
			}
			return fail.NotFoundError("share '%s' not found in server '%s' metadata", shareRef, rh.GetName())
		})
	})
	if err != nil {
		return nil, err
	}

	return hostShare, nil
}

// GetVolumes returns information about volumes attached to the host
func (rh host) GetVolumes(task concurrency.Task) (*propertiesv1.HostVolumes, fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task == nil {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}

	var hvV1 *propertiesv1.HostVolumes
	err := rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.VolumesV1, func(clonable data.Clonable) fail.Error {
			var ok bool
			hvV1, ok = clonable.(*propertiesv1.HostVolumes)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.getVolumes' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return hvV1, nil
}

// getVolumes returns information about volumes attached to the host
func (rh host) getVolumes(task concurrency.Task) *propertiesv1.HostVolumes {
	out, _ := rh.GetVolumes(task)
	return out
}

// // GetAttachedVolume returns information about where and how the volume referenced is attached to the host
// func (objh *host) GetAttachedVolume(task concurrency.Task, volumeRef string) (*propertiesv1.HostLocalMount, fail.Error) {
// 	if objh.IsNUll() {
// 		return nil, fail.InvalidInstanceError()
// 	}
// 	if task == nil {
// 		return nil, fail.InvalidParameterError("task", "cannot be nil")
// 	}
// 	if volumeRef == "" {
// 		return nil, fail.InvalidParameterError("volumeRef", "cannot be empty string")
// 	}

// 	var mount *propertiesv1.HostMount
// 	err := objh.Inspect(task, func(clonable data.Clonable, props *serialize.JSONProperties) error {
// 		var hostVolume *propertiesv1.HostVolume
// 		innerErr := props.Inspect(hostproperty.HostVolumesV1, func(clonable data.Clonable) error {
// 			vaV1, ok := clonable.(*propertiesv1.HostVolumes)
// 			if !ok {
// 				return fail.InconsistentError("'*propertiesv1.HostVolumess' expected, '%s' provided", reflect.TypeOf(clonable).String())
// 			}
// 			hostVolume, ok = vaV1.VolumesByID[volumeRef]
// 			if !ok {
// 				var ref string
// 				ref, ok = vaV1.VolumesByName[volumeRef]
// 				hostVolume, ok = vaV1.VolumesByID[ref]
// 			}
// 			if !ok {
// 				return fail.NotFoundError("failed to find a volume referenced by '%s' attached to host '%s'", volumeRef, objh.GetName())
// 			}
// 			return nil
// 		})
// 		if innerErr != nil {
// 			return innerErr
// 		}

// 		return props.Inspect(hostproperty.HostLocalMountV1, func(clonable data.Clonable) error {
// 			hlmV1, ok := clonable.(*propertiesv1.HostLocalMount)
// 			if !ok {
// 				return fail.InconsistentError("'*propertiesv1.HostMount' expected, '%s' provided", reflect.TypeOf(clonable).String())
// 			}
// 			mount, ok := hlmV1.ByDevice[hostVolume.Device]
// 			return nil
// 		})
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return mount, nil
// }

// Start starts the host
func (rh host) Start(task concurrency.Task) (xerr fail.Error) {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	fail.OnPanic(&xerr)

	hostName := rh.GetName()
	hostID := rh.GetID()

	svc := rh.GetService()
	if xerr = svc.StartHost(hostID); xerr != nil {
		return xerr
	}

	xerr = retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			return svc.WaitHostState(hostID, hoststate.STARTED, temporal.GetHostTimeout())
		},
		5*time.Minute,
	)
	if xerr != nil {
		return fail.Wrap(xerr, "timeout waiting host '%s' to be started", hostName)
	}
	return nil
}

// Stop stops the host
func (rh host) Stop(task concurrency.Task) (xerr fail.Error) {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}

	hostName := rh.GetName()
	hostID := rh.GetID()

	svc := rh.GetService()
	if xerr = svc.StopHost(hostID); xerr != nil {
		return xerr
	}

	xerr = retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			return svc.WaitHostState(hostID, hoststate.STOPPED, temporal.GetHostTimeout())
		},
		// FIXME: static value
		5*time.Minute,
	)
	if xerr != nil {
		return fail.Wrap(xerr, "timeout waiting host '%s' to be stopped", hostName)
	}
	return nil
}

// Reboot reboots the host
func (rh host) Reboot(task concurrency.Task) fail.Error {
	if xerr := rh.Stop(task); xerr != nil {
		return xerr
	}
	return rh.Start(task)
}

// Resize ...
// not yet implemented
func (rh *host) Resize(hostSize abstract.HostSizingRequirements) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	return fail.NotImplementedError("Host.Resize() not yet implemented")
}

// AddFeature handles 'safescale host add-feature <host name or id> <feature name>'
func (rh *host) AddFeature(task concurrency.Task, name string, vars data.Map, settings resources.FeatureSettings) (outcomes resources.Results, xerr fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task == nil {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}
	if name == "" {
		return nil, fail.InvalidParameterError("name", "cannot be empty string")
	}

	tracer := debug.NewTracer(task, tracing.ShouldTrace("resources.host"), "(%s)", name).Entering()
	defer tracer.Exiting()

	feat, xerr := NewFeature(task, name)
	if xerr != nil {
		return nil, xerr
	}
	xerr = rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		var innerXErr fail.Error
		outcomes, innerXErr = feat.Add(rh, vars, settings)
		if innerXErr != nil {
			return innerXErr
		}

		// updates HostFeatures property for host
		return props.Alter(task, hostproperty.FeaturesV1, func(clonable data.Clonable) fail.Error {
			hostFeaturesV1, ok := clonable.(*propertiesv1.HostFeatures)
			if !ok {
				return fail.InconsistentError("expected '*propertiesv1.HostFeatures', received '%s'", reflect.TypeOf(clonable))
			}
			requires, innerXErr := feat.GetRequirements()
			if innerXErr != nil {
				return innerXErr
			}
			hostFeaturesV1.Installed[name] = &propertiesv1.HostInstalledFeature{
				HostContext: true,
				Requires:    requires,
			}
			return nil
		})
	})
	if xerr != nil {
		return nil, xerr
	}
	return outcomes, nil
}

// CheckFeature ...
func (rh host) CheckFeature(task concurrency.Task, name string, vars data.Map, settings resources.FeatureSettings) (resources.Results, fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task == nil {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}
	if name == "" {
		return nil, fail.InvalidParameterError("featureName", "cannot be empty string")
	}

	tracer := debug.NewTracer(task, tracing.ShouldTrace("resources.host"), "(%s)", name).Entering()
	defer tracer.Exiting()

	feat, xerr := NewFeature(task, name)
	if xerr != nil {
		return nil, xerr
	}

	// Wait for SSH service on remote host first
	// ssh, err := mh.GetSSHConfig(task)
	// if err != nil {
	// 	return srvutils.ThrowErr(err)
	// }
	// _, err = ssh.WaitServerReady(2 * time.Minute)
	// if err != nil {
	// 	return srvutils.ThrowErr(err)
	// }

	return feat.Check(&rh, vars, settings)
}

// DeleteFeature handles 'safescale host delete-feature <host name> <feature name>'
func (rh *host) DeleteFeature(task concurrency.Task, name string, vars data.Map, settings resources.FeatureSettings) (resources.Results, fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task == nil {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}
	if name == "" {
		return nil, fail.InvalidParameterError("featureName", "cannot be empty string")
	}

	tracer := debug.NewTracer(task, false /*Trace.Host, */, "(%s)", name).Entering()
	defer tracer.Exiting()

	feat, xerr := NewFeature(task, name)
	if xerr != nil {
		return nil, xerr
	}

	// // Wait for SSH service on remote host first
	// ssh, err := mh.GetSSHConfig(task)
	// if err != nil {
	// 	return srvutils.ThrowErr(err)
	// }
	// _, err = ssh.WaitServerReady(2 * time.Minute)
	// if err != nil {
	// 	return srvutils.ThrowErr(err)
	// }

	xerr = rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		outcomes, innerXErr := feat.Remove(rh, vars, settings)
		if innerXErr != nil {
			return fail.NewError(innerXErr, nil, "error uninstalling feature '%s' on '%s'", name, rh.GetName())
		}
		if !outcomes.Successful() {
			msg := fmt.Sprintf("failed to delete feature '%s' from host '%s'", name, rh.GetName())
			tracer.Trace(strprocess.Capitalize(msg) + ":\n" + outcomes.AllErrorMessages())
			return fail.NewError(msg)
		}

		// updates HostFeatures property for host
		return props.Alter(task, hostproperty.FeaturesV1, func(clonable data.Clonable) fail.Error {
			hostFeaturesV1, ok := clonable.(*propertiesv1.HostFeatures)
			if !ok {
				return fail.InconsistentError("expected '*propertiesv1.HostFeatures', received '%s'", reflect.TypeOf(clonable))
			}
			delete(hostFeaturesV1.Installed, name)
			return nil
		})
	})
	return nil, xerr
}

// TargetType returns the type of the target.
// satisfies install.Targetable interface.
func (rh host) TargetType() featuretargettype.Enum {
	if rh.IsNull() {
		return featuretargettype.UNKNOWN
	}
	return featuretargettype.HOST
}

// GetPublicIP returns the public IP address of the host
func (rh host) GetPublicIP(task concurrency.Task) (ip string, xerr fail.Error) {
	ip = ""
	if rh.IsNull() {
		return "", fail.InvalidInstanceError()
	}
	if task == nil {
		return "", fail.InvalidParameterError("task", "cannot be nil")
	}

	return rh.getPublicIP(task), nil
}

// getPublicIP returns the public IP address of the host
// To be used when rh is notoriously not nil
func (rh host) getPublicIP(task concurrency.Task) string {
	if rh.IsNull() || task == nil {
		return ""
	}

	rh.SafeRLock(task)
	defer rh.SafeRUnlock(task)
	return rh.publicIP
}

// GetPrivateIP returns the private IP of the host on its default Networking
func (rh host) GetPrivateIP(task concurrency.Task) (ip string, _ fail.Error) {
	ip = ""
	if rh.IsNull() {
		return ip, fail.InvalidInstanceError()
	}
	if task == nil {
		return ip, fail.InvalidParameterError("task", "cannot be nil")
	}

	return rh.getPrivateIP(task), nil
}

// getPrivateIP returns the private IP of the host on its default Networking
// To be used when rh is notoriously not nil
func (rh host) getPrivateIP(task concurrency.Task) string {
	if rh.IsNull() || task == nil {
		return ""
	}

	rh.SafeRLock(task)
	defer rh.SafeRUnlock(task)
	return rh.privateIP
}

// GetPrivateIPOnSubnet returns the private IP of the host on its default Networking
func (rh host) GetPrivateIPOnSubnet(task concurrency.Task, subnetID string) (ip string, xerr fail.Error) {
	ip = ""
	if rh.IsNull() {
		return ip, fail.InvalidInstanceError()
	}
	if task == nil {
		return ip, fail.InvalidParameterError("task", "cannot be nil")
	}
	if subnetID == "" {
		return ip, fail.InvalidParameterError("subnetID", "cannot be empty string")
	}

	xerr = rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		if props.Lookup(hostproperty.NetworkV2) {
			return props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
				hostNetworkV2, ok := clonable.(*propertiesv2.HostNetworking)
				if !ok {
					return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}
				if ip, ok = hostNetworkV2.IPv4Addresses[subnetID]; !ok {
					return fail.InvalidRequestError("host '%s' does not have an IP address on subnet '%s'", rh.GetName(), subnetID)
				}
				return nil
			})
		}
		return props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hostNetworkV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			if ip, ok = hostNetworkV2.IPv4Addresses[subnetID]; !ok {
				return fail.InvalidRequestError("host '%s' does not have an IP address on subnet '%s'", rh.GetName(), subnetID)
			}
			return nil
		})
	})
	return ip, xerr
}

//// getPrivateIPOnSubnet returns the private IP of the host on request subnet
//// To be used when rh is notoriously not nil
//func (rh *host) getPrivateIPOnSubnet(task concurrency.Task, networkID string) string {
//	ip, _ := rh.GetPrivateIPOnSubnet(task, networkID)
//	return ip
//}

// GetAccessIP returns the IP to reach the host
func (rh *host) GetAccessIP(task concurrency.Task) (ip string, _ fail.Error) {
	ip = ""
	if rh.IsNull() {
		return ip, fail.InvalidInstanceError()
	}
	if task == nil {
		return ip, fail.InvalidParameterError("task", "cannot be nil")
	}

	return rh.getAccessIP(task), nil
}

// getAccessIP returns the IP to reach the host
// To be used when rh is notoriously not nil
func (rh *host) getAccessIP(task concurrency.Task) string {
	if rh.IsNull() {
		return ""
	}

	rh.SafeRLock(task)
	defer rh.SafeRUnlock(task)
	return rh.accessIP
}

// InstallMethods returns a list of installation methods useable on the target, ordered from upper to lower preference (1 = highest preference)
//
// satisfies interface install.Targetable
func (rh host) InstallMethods(task concurrency.Task) map[uint8]installmethod.Enum {
	if rh.IsNull() {
		logrus.Error(fail.InvalidInstanceError().Error())
		return map[uint8]installmethod.Enum{}
	}
	if task == nil {
		logrus.Error(fail.InvalidParameterError("task", "cannot be nil").Error())
		return map[uint8]installmethod.Enum{}
	}

	rh.SafeLock(task)
	defer rh.SafeUnlock(task)

	if rh.installMethods == nil {
		rh.installMethods = map[uint8]installmethod.Enum{}

		_ = rh.Inspect(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
			// props, inErr := rh.properties(task)
			// if inErr != nil {
			// 	return inErr
			// }

			// Ignore error in this special case; will fallback to use bash method if cannot determine operating system type and flavor
			var index uint8
			_ = props.Inspect(task, hostproperty.SystemV1, func(clonable data.Clonable) fail.Error {
				systemV1, ok := clonable.(*propertiesv1.HostSystem)
				if !ok {
					logrus.Error(fail.InconsistentError("'*propertiesv1.HostSystem' expected, '%s' provided", reflect.TypeOf(clonable).String()))
				}
				if systemV1.Type == "linux" {
					switch systemV1.Flavor {
					case "centos", "redhat":
						index++
						rh.installMethods[index] = installmethod.Yum
					case "debian":
						fallthrough
					case "ubuntu":
						index++
						rh.installMethods[index] = installmethod.Apt
					case "fedora", "rhel":
						index++
						rh.installMethods[index] = installmethod.Dnf
					}
				}
				return nil
			})
			index++
			rh.installMethods[index] = installmethod.Bash
			return nil
		})
	}
	return rh.installMethods
}

// GetShares returns the information about the shares hosted by the host
func (rh host) GetShares(task concurrency.Task) (shares *propertiesv1.HostShares, xerr fail.Error) {
	shares = &propertiesv1.HostShares{}
	if rh.IsNull() {
		return shares, fail.InvalidInstanceError()
	}
	if task == nil {
		return shares, fail.InvalidParameterError("task", "cannot be nil")
	}

	xerr = rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.SharesV1, func(clonable data.Clonable) fail.Error {
			hostSharesV1, ok := clonable.(*propertiesv1.HostShares)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostShares' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			shares = hostSharesV1
			return nil
		})
	})
	return shares, xerr
}

// Shares returns the information about the shares of the host
// Intented to be used when objn is notoriously not nil (because previously checked)
func (rh host) getShares(task concurrency.Task) *propertiesv1.HostShares {
	shares, _ := rh.GetShares(task)
	return shares
}

// GetMounts returns the information abouts the mounts of the host
func (rh host) GetMounts(task concurrency.Task) (mounts *propertiesv1.HostMounts, xerr fail.Error) {
	mounts = nil
	if rh.IsNull() {
		return mounts, fail.InvalidInstanceError()
	}
	if task == nil {
		return mounts, fail.InvalidParameterError("task", "cannot be nil")
	}

	xerr = rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.SharesV1, func(clonable data.Clonable) fail.Error {
			hostMountsV1, ok := clonable.(*propertiesv1.HostMounts)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostMounts' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			mounts = hostMountsV1
			return nil
		})
	})
	return mounts, xerr
}

// getMounts returns the information about the mounts of the host
// Intended to be used when objh is notoriously not nil (because previously checked)
func (rh host) getMounts(task concurrency.Task) *propertiesv1.HostMounts {
	mounts, _ := rh.GetMounts(task)
	return mounts
}

// InstalledFeatures returns a list of installed features
//
// satisfies interface install.Targetable
func (rh host) InstalledFeatures(task concurrency.Task) []string {
	var list []string
	return list
}

// ComplementFeatureParameters configures parameters that are appropriate for the target
// satisfies interface install.Targetable
func (rh host) ComplementFeatureParameters(task concurrency.Task, v data.Map) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	if v == nil {
		return fail.InvalidParameterError("v", "cannot be nil")
	}

	v["ShortHostname"] = rh.GetName()
	domain := ""
	xerr := rh.Inspect(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.DescriptionV1, func(clonable data.Clonable) fail.Error {
			hostDescriptionV1, ok := clonable.(*propertiesv1.HostDescription)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostDescription' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			domain = hostDescriptionV1.Domain

			if domain != "" {
				domain = "." + domain
			}
			return nil
		})
	})
	if xerr != nil {
		return xerr
	}

	v["Hostname"] = rh.GetName() + domain

	v["HostIP"] = rh.getPrivateIP(task)
	v["getPublicIP"] = rh.getPublicIP(task)

	if _, ok := v["Username"]; !ok {
		v["Username"] = abstract.DefaultUser
	}

	rs, xerr := rh.GetDefaultSubnet(task)
	if xerr != nil {
		return xerr
	}

	rgw, xerr := rs.GetGateway(task, true)
	if xerr != nil {
		return xerr
	}
	rgwi := rgw.(*host)
	v["PrimaryGatewayIP"] = rgwi.getPrivateIP(task)
	v["GatewayIP"] = v["PrimaryGatewayIP"] // legacy
	v["PrimaryPublicIP"] = rgwi.getPublicIP(task)

	rgw, xerr = rs.GetGateway(task, false)
	if xerr != nil {
		if _, ok := xerr.(*fail.ErrNotFound); !ok {
			return xerr
		}
	} else {
		rgwi = rgw.(*host)
		v["SecondaryGatewayIP"] = rgwi.getPrivateIP(task)
		v["SecondaryPublicIP"] = rgwi.getPublicIP(task)
	}

	if v["getEndpointIP"], xerr = rs.GetEndpointIP(task); xerr != nil {
		return xerr
	}
	v["getPublicIP"] = v["getEndpointIP"]
	if v["getDefaultRouteIP"], xerr = rs.GetDefaultRouteIP(task); xerr != nil {
		return xerr
	}
	return nil
}

// IsClusterMember returns true if the host is member of a cluster
func (rh host) IsClusterMember(task concurrency.Task) (yes bool, xerr fail.Error) {
	yes = false
	if rh.IsNull() {
		return yes, fail.InvalidInstanceError()
	}
	if task == nil {
		return yes, fail.InvalidParameterError("task", "cannot be nil")
	}

	xerr = rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.ClusterMembershipV1, func(clonable data.Clonable) fail.Error {
			hostClusterMembershipV1, ok := clonable.(*propertiesv1.HostClusterMembership)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostClusterMembership' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			yes = hostClusterMembershipV1.Cluster != ""
			return nil
		})
	})
	return yes, xerr
}

// IsGateway tells if the host acts as a gateway for a Subnet
func (rh host) IsGateway(task concurrency.Task) (bool, fail.Error) {
	if rh.IsNull() {
		return false, fail.InvalidInstanceError()
	}

	var state bool
	xerr := rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hnV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			state = hnV2.IsGateway
			return nil
		})
	})
	if xerr != nil {
		return false, xerr
	}
	return state, nil
}

// PushStringToFile creates a file 'filename' on remote 'host' with the content 'content'
func (rh host) PushStringToFile(task concurrency.Task, content string, filename string, owner, mode string) (xerr fail.Error) {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	if content == "" {
		return fail.InvalidParameterError("content", "cannot be empty string")
	}
	if filename == "" {
		return fail.InvalidParameterError("filename", "cannot be empty string")
	}

	hostName := rh.GetName()
	f, xerr := system.CreateTempFileFromString(content, 0600)
	if xerr != nil {
		return fail.Wrap(xerr, "failed to create temporary file")
	}
	to := fmt.Sprintf("%s:%s", hostName, filename)
	deleted := false
	retryErr := retry.WhileUnsuccessful(
		func() error {
			var (
				retcode   int
				innerXErr error
			)
			retcode, _, _, innerXErr = rh.Push(task, f.Name(), filename, owner, mode, temporal.GetExecutionTimeout())
			if innerXErr != nil {
				return innerXErr
			}
			if retcode != 0 {
				// If retcode == 1 (general copy error), retry. It may be a temporary subnet incident
				if retcode == 1 && !deleted {
					// File may exist on target, try to remove it
					if _, _, _, innerXErr = rh.Run(task, "sudo rm -f "+filename, outputs.COLLECT, temporal.GetConnectionTimeout(), temporal.GetExecutionTimeout()); innerXErr == nil {
						deleted = true
					}
					return fail.NewError("file may have existing on remote with inappropriate access rights, deleted it and now retrying")
				}
				if system.IsSCPRetryable(retcode) {
					xerr = fail.NewError("failed to copy temporary file to '%s' (retcode: %d=%s)", to, retcode, system.SCPErrorString(retcode))
				}
			}
			return nil
		},
		1*time.Second,
		2*time.Minute,
	)
	_ = os.Remove(f.Name())
	if retryErr != nil {
		if _, ok := retryErr.(*retry.ErrTimeout); ok {
			return fail.Wrap(retryErr, "timeout trying to copy temporary file to '%s'", to)
		}
		return xerr
	}

	cmd := ""
	if owner != "" {
		cmd += `sudo chown ` + owner + ` '` + filename + `' ;`
	}
	if mode != "" {
		cmd += `sudo chmod ` + mode + ` '` + filename + `'`
	}
	if cmd != "" {
		retryErr = retry.WhileUnsuccessful(
			func() error {
				retcode, stdout, _, innerXErr := rh.Run(task, cmd, outputs.COLLECT, temporal.GetConnectionTimeout(), temporal.GetExecutionTimeout())
				if innerXErr != nil {
					// on error, innerXErr already has annotations "retcode" and "stderr", we need to add stdout
					_ = innerXErr.Annotate("stdout", stdout)
					return innerXErr
				}
				if retcode != 0 {
					xerr = fail.NewError("failed to change rights of file '%s' (retcode=%d)", to, retcode)
				}
				return nil
			},
			2*time.Second,
			1*time.Minute,
		)
		if retryErr != nil {
			switch retryErr.(type) {
			case *retry.ErrTimeout:
				return xerr
			default:
				return fail.Wrap(retryErr, "failed to change rights of file '%s' on host '%s'", filename, hostName)
			}
		}
	}
	return nil
}

// GetDefaultSubnet returns the Networking instance corresponding to host default subnet
func (rh host) GetDefaultSubnet(task concurrency.Task) (rs resources.Subnet, xerr fail.Error) {
	nullSubnet := nullSubnet()
	if rh.IsNull() {
		return nullSubnet, fail.InvalidInstanceError()
	}
	if task == nil {
		return nullSubnet, fail.InvalidParameterError("task", "cannot be nil")
	}

	var innerXErr fail.Error
	xerr = rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		if props.Lookup(hostproperty.NetworkV2) {
			return props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
				networkV2, ok := clonable.(*propertiesv2.HostNetworking)
				if !ok {
					return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
				}
				rs, innerXErr = LoadSubnet(task, rh.GetService(), "", networkV2.DefaultSubnetID)
				if innerXErr != nil {
					return innerXErr
				}
				return nil
			})
		}
		return props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
			hostNetworkV2, ok := clonable.(*propertiesv2.HostNetworking)
			if !ok {
				return fail.InconsistentError("'*propertiesv2.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			rs, innerXErr = LoadSubnet(task, rh.GetService(), "", hostNetworkV2.DefaultSubnetID)
			if innerXErr != nil {
				return innerXErr
			}
			return nil
		})
	})
	if xerr != nil {
		return nullSubnet, xerr
	}

	return rs, nil
}

// ToProtocol convert an resources.Host to protocol.Host
func (rh host) ToProtocol(task concurrency.Task) (ph *protocol.Host, xerr fail.Error) {
	if rh.IsNull() {
		return nil, fail.InvalidInstanceError()
	}
	if task == nil {
		return nil, fail.InvalidParameterError("task", "cannot be nil")
	}

	defer fail.OnPanic(&xerr)

	var (
		ahc *abstract.HostCore
		//hostNetworkV2 *propertiesv2.HostNetworking
		hostSizingV1  *propertiesv1.HostSizing
		hostVolumesV1 *propertiesv1.HostVolumes
		volumes       []string
	)

	publicIP := rh.getPublicIP(task)
	privateIP := rh.getPrivateIP(task)

	xerr = rh.Inspect(task, func(clonable data.Clonable, props *serialize.JSONProperties) fail.Error {
		var ok bool
		ahc, ok = clonable.(*abstract.HostCore)
		if !ok {
			return fail.InconsistentError("'*abstract.HostCore' expected, '%s' provided", reflect.TypeOf(clonable).String())
		}
		//return props.Inspect(task, hostproperty.NetworkV2, func(clonable data.Clonable) fail.Error {
		//	hostNetworkV2, ok = clonable.(*propertiesv2.HostNetworking)
		//	if !ok {
		//		return fail.InconsistentError("'*propertiesv1.HostNetworking' expected, '%s' provided", reflect.TypeOf(clonable).String)
		//	}
		return props.Inspect(task, hostproperty.SizingV1, func(clonable data.Clonable) fail.Error {
			hostSizingV1, ok = clonable.(*propertiesv1.HostSizing)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSizing' expected, '%s' provided", reflect.TypeOf(clonable).String)
			}
			return props.Inspect(task, hostproperty.VolumesV1, func(clonable data.Clonable) fail.Error {
				hostVolumesV1, ok = clonable.(*propertiesv1.HostVolumes)
				if !ok {
					return fail.InconsistentError("'*propertiesv1.HostVolumes' expected, '%s' provided", reflect.TypeOf(clonable).String)
				}

				volumes = make([]string, 0, len(hostVolumesV1.VolumesByName))
				for _, v := range hostVolumesV1.VolumesByName {
					volumes = append(volumes, v)
				}
				return nil
			})
		})
		//})
	})
	if xerr != nil {
		return ph, xerr
	}

	ph = &protocol.Host{
		Cpu:  int32(hostSizingV1.AllocatedSize.Cores),
		Disk: int32(hostSizingV1.AllocatedSize.DiskSize),
		//GatewayId:           hostNetworkV2.DefaultGatewayID,
		Id:                  ahc.ID,
		PublicIp:            publicIP,
		PrivateIp:           privateIP,
		Name:                ahc.Name,
		PrivateKey:          ahc.PrivateKey,
		Password:            ahc.Password,
		Ram:                 hostSizingV1.AllocatedSize.RAMSize,
		State:               protocol.HostState(ahc.LastState),
		AttachedVolumeNames: volumes,
	}
	return ph, nil
}

// BindSecurityGroup binds a security group to the host; if enabled is true, apply it immediately
func (rh *host) BindSecurityGroup(task concurrency.Task, sg resources.SecurityGroup, enable resources.SecurityGroupActivation) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	if sg.IsNull() {
		return fail.InvalidParameterError("sg", "cannot be null value of 'SecurityGroup'")
	}

	return rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Alter(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) fail.Error {
			hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}

			sgID := sg.GetID()
			// If the Security Group is already bound to the host with the exact same state, consider as a success
			if v, ok := hsgV1.ByID[sgID]; ok && v.Disabled == !bool(enable) {
				return nil
			}

			// Not found, add it
			item := &propertiesv1.SecurityGroupBond{
				ID:       sgID,
				Name:     sg.GetName(),
				Disabled: bool(!enable),
			}
			hsgV1.ByID[sgID] = item
			hsgV1.ByName[item.Name] = item.ID

			// If enabled, apply it
			innerXErr := sg.BindToHost(task, rh, enable, resources.MarkSecurityGroupAsSupplemental)
			if innerXErr != nil {
				switch innerXErr.(type) {
				case *fail.ErrDuplicate:
				// already bound, success
				default:
					return innerXErr
				}
			}
			return nil
		})
	})
}

// UnbindSecurityGroup unbinds a security group from the host
func (rh *host) UnbindSecurityGroup(task concurrency.Task, sg resources.SecurityGroup) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	if sg.IsNull() {
		return fail.InvalidParameterError("sg", "cannot be null value of 'SecurityGroup'")
	}

	return rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Alter(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) fail.Error {
			hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}

			sgID := sg.GetID()
			// Check if the security group is listed for the host
			found := false
			for k, v := range hsgV1.ByID {
				if k == sgID {
					if v.FromSubnet {
						return fail.InvalidRequestError("cannot unbind a security group from host when from subnet")
					}
					found = true
					break
				}
			}
			// If not found, consider request successful
			if !found {
				return nil
			}

			// unbind security group from host on remote service side
			if innerXErr := sg.UnbindFromHost(task, rh); innerXErr != nil {
				return innerXErr
			}

			// found, delete it from properties
			delete(hsgV1.ByID, sgID)
			delete(hsgV1.ByName, sg.GetName())
			return nil

		})
	})
}

// ListSecurityGroups returns a slice of security groups binded to host
func (rh *host) ListSecurityGroups(task concurrency.Task, state securitygroupstate.Enum) (list []*propertiesv1.SecurityGroupBond, _ fail.Error) {
	var nullList []*propertiesv1.SecurityGroupBond
	if rh.IsNull() {
		return nullList, fail.InvalidInstanceError()
	}
	if task == nil {
		return nullList, fail.InvalidParameterError("task", "cannot be nil")
	}

	return list, rh.Inspect(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) fail.Error {
			hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}
			list = filterBondsByKind(hsgV1.ByID, state)
			return nil
		})
	})
}

// EnableSecurityGroup enables a bound security group to host by applying its rules
func (rh *host) EnableSecurityGroup(task concurrency.Task, sg resources.SecurityGroup) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	if sg.IsNull() {
		return fail.InvalidParameterError("sg", "cannot be null value of 'SecurityGroup'")
	}

	return rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Inspect(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) fail.Error {
			hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}

			sgID := sg.GetID()
			// First check if the security group is not already registered for the host with the exact same state
			var found bool
			for k := range hsgV1.ByID {
				if k == sgID {
					found = true
				}
			}
			if !found {
				return fail.NotFoundError("security group '%s' is not bound to host '%s'", sg.GetName(), rh.GetID())
			}

			// Bind the security group on provider side; if already bound (*fail.ErrDuplicate), consider as a success
			if innerXErr := sg.GetService().BindSecurityGroupToHost(sgID, rh.GetID()); innerXErr != nil {
				switch innerXErr.(type) {
				case *fail.ErrDuplicate:
					return nil
				default:
					return innerXErr
				}
			}

			// found and updated, update metadata
			hsgV1.ByID[sgID].Disabled = false
			return nil
		})
	})
}

// DisableSecurityGroup disables a binded security group to host
func (rh *host) DisableSecurityGroup(task concurrency.Task, sg resources.SecurityGroup) fail.Error {
	if rh.IsNull() {
		return fail.InvalidInstanceError()
	}
	if task == nil {
		return fail.InvalidParameterError("task", "cannot be nil")
	}
	if sg.IsNull() {
		return fail.InvalidParameterError("sg", "cannot be null value of 'SecurityGroup'")
	}

	return rh.Alter(task, func(_ data.Clonable, props *serialize.JSONProperties) fail.Error {
		return props.Alter(task, hostproperty.SecurityGroupsV1, func(clonable data.Clonable) fail.Error {
			hsgV1, ok := clonable.(*propertiesv1.HostSecurityGroups)
			if !ok {
				return fail.InconsistentError("'*propertiesv1.HostSecurityGroups' expected, '%s' provided", reflect.TypeOf(clonable).String())
			}

			sgID := sg.GetID()
			// First check if the security group is not already registered for the host with the exact same state
			var found bool
			for k := range hsgV1.ByID {
				if k == sgID {
					found = true
				}
			}
			if !found {
				return fail.NotFoundError("security group '%s' is not bound to host '%s'", sg.GetName(), sg.GetID())
			}

			// Bind the security group on provider side; if security group not binded, consider as a success
			if innerXErr := sg.GetService().UnbindSecurityGroupFromHost(sgID, rh.GetID()); innerXErr != nil {
				switch innerXErr.(type) {
				case *fail.ErrNotFound:
					return nil
				default:
					return innerXErr
				}
			}

			// found, update properties
			hsgV1.ByID[sgID].Disabled = true
			return nil
		})
	})
}
