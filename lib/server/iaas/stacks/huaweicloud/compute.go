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

package huaweicloud

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/CS-SI/SafeScale/lib/utils"
	"github.com/CS-SI/SafeScale/lib/utils/debug"

	"github.com/davecgh/go-spew/spew"
	"github.com/pengux/check"
	"github.com/sirupsen/logrus"

	gc "github.com/gophercloud/gophercloud"
	nics "github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/attachinterfaces"
	exbfv "github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/floatingips"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/pagination"

	"github.com/CS-SI/SafeScale/lib/utils/data"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
	"github.com/CS-SI/SafeScale/lib/utils/temporal"

	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract"
	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/enums/hostproperty"
	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/enums/hoststate"
	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/enums/ipversion"
	converters "github.com/CS-SI/SafeScale/lib/server/iaas/abstract/properties"
	propsv1 "github.com/CS-SI/SafeScale/lib/server/iaas/abstract/properties/v1"
	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/userdata"
	"github.com/CS-SI/SafeScale/lib/server/iaas/stacks/openstack"
	"github.com/CS-SI/SafeScale/lib/utils/retry"
)

type blockDevice struct {
	// SourceType must be one of: "volume", "snapshot", "image", or "blank".
	SourceType exbfv.SourceType `json:"source_type" required:"true"`

	// UUID is the unique identifier for the existing volume, snapshot, or
	// image (see above).
	UUID string `json:"uuid,omitempty"`

	// BootIndex is the boot index. It defaults to 0.
	BootIndex string `json:"boot_index,omitempty"`

	// DeleteOnTermination specifies whether or not to delete the attached volume
	// when the server is deleted. Defaults to `false`.
	DeleteOnTermination bool `json:"delete_on_termination"`

	// DestinationType is the type that gets created. Possible values are "volume"
	// and "local".
	DestinationType exbfv.DestinationType `json:"destination_type,omitempty"`

	// GuestFormat specifies the format of the block device.
	GuestFormat string `json:"guest_format,omitempty"`

	// VolumeSize is the size of the volume to create (in gigabytes). This can be
	// omitted for existing volumes.
	VolumeSize int `json:"volume_size,omitempty"`

	// Type of volume
	VolumeType string `json:"volume_type,omitempty"`
}

// CreateOptsExt is a structure that extends the server `CreateOpts` structure
// by allowing for a block device mapping.
type bootdiskCreateOptsExt struct {
	servers.CreateOptsBuilder
	BlockDevice []blockDevice `json:"block_device_mapping_v2,omitempty"`
}

// ToServerCreateMap adds the block device mapping option to the base server
// creation options.
func (opts bootdiskCreateOptsExt) ToServerCreateMap() (map[string]interface{}, fail.Error) {
	base, err := opts.CreateOptsBuilder.ToServerCreateMap()
	if err != nil {
		return nil, err
	}

	if len(opts.BlockDevice) == 0 {
		err := gc.ErrMissingInput{}
		err.Argument = "bootfromvolume.CreateOptsExt.BlockDevice"
		return nil, err
	}

	serverMap := base["server"].(map[string]interface{})

	blkDevices := make([]map[string]interface{}, len(opts.BlockDevice))

	for i, bd := range opts.BlockDevice {
		b, err := gc.BuildRequestBody(bd, "")
		if err != nil {
			return nil, err
		}
		blkDevices[i] = b
	}
	serverMap["block_device_mapping_v2"] = blkDevices

	return base, nil
}

type serverCreateOpts struct {
	// Name is the name to assign to the newly launched server.
	Name string `json:"name" required:"true"`

	// ImageRef [optional; required if ImageName is not provided] is the ID or
	// full URL to the image that contains the server's OS and initial state.
	// Also optional if using the boot-from-volume extension.
	ImageRef string `json:"imageRef,omitempty"`

	// ImageName [optional; required if ImageRef is not provided] is the name of
	// the image that contains the server's OS and initial state.
	// Also optional if using the boot-from-volume extension.
	ImageName string `json:"-,omitempty"`

	// FlavorRef [optional; required if FlavorName is not provided] is the ID or
	// full URL to the flavor that describes the server's specs.
	FlavorRef string `json:"flavorRef"`

	// FlavorName [optional; required if FlavorRef is not provided] is the name of
	// the flavor that describes the server's specs.
	FlavorName string `json:"-"`

	// SecurityGroups lists the names of the security groups to which this server
	// should belong.
	SecurityGroups []string `json:"-"`

	// UserData contains configuration information or scripts to use upon launch.
	// Create will base64-encode it for you, if it isn't already.
	UserData []byte `json:"-"`

	// AvailabilityZone in which to launch the server.
	AvailabilityZone string `json:"availability_zone,omitempty"`

	// Networks dictates how this server will be attached to available networks.
	// By default, the server will be attached to all isolated networks for the
	// tenant.
	Networks []servers.Network `json:"-"`

	// Metadata contains key-value pairs (up to 255 bytes each) to attach to the
	// server.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Personality includes files to inject into the server at launch.
	// Create will base64-encode file contents for you.
	Personality servers.Personality `json:"personality,omitempty"`

	// ConfigDrive enables metadata injection through a configuration drive.
	ConfigDrive *bool `json:"config_drive,omitempty"`

	// AdminPass sets the root user password. If not set, a randomly-generated
	// password will be created and returned in the response.
	AdminPass string `json:"adminPass,omitempty"`

	// AccessIPv4 specifies an IPv4 address for the instance.
	AccessIPv4 string `json:"accessIPv4,omitempty"`

	// AccessIPv6 pecifies an IPv6 address for the instance.
	AccessIPv6 string `json:"accessIPv6,omitempty"`

	// ServiceClient will allow calls to be made to retrieve an image or
	// flavor ID by name.
	ServiceClient *gc.ServiceClient `json:"-"`
}

// ToServerCreateMap assembles a request body based on the contents of a
// CreateOpts.
func (opts serverCreateOpts) ToServerCreateMap() (map[string]interface{}, fail.Error) {
	sc := opts.ServiceClient
	opts.ServiceClient = nil
	b, err := gc.BuildRequestBody(opts, "")
	if err != nil {
		return nil, err
	}

	if opts.UserData != nil {
		var userData string
		if _, err := base64.StdEncoding.DecodeString(string(opts.UserData)); err != nil {
			userData = base64.StdEncoding.EncodeToString(opts.UserData)
		} else {
			userData = string(opts.UserData)
		}
		// logrus.Debugf("Base64 encoded userdata size = %d bytes", len(userData))
		b["user_data"] = &userData
	}

	if len(opts.SecurityGroups) > 0 {
		securityGroups := make([]map[string]interface{}, len(opts.SecurityGroups))
		for i, groupName := range opts.SecurityGroups {
			securityGroups[i] = map[string]interface{}{"name": groupName}
		}
		b["security_groups"] = securityGroups
	}

	if len(opts.Networks) > 0 {
		networks := make([]map[string]interface{}, len(opts.Networks))
		for i, network := range opts.Networks {
			networks[i] = make(map[string]interface{})
			if network.UUID != "" {
				networks[i]["uuid"] = network.UUID
			}
			if network.Port != "" {
				networks[i]["port"] = network.Port
			}
			if network.FixedIP != "" {
				networks[i]["fixed_ip"] = network.FixedIP
			}
		}
		b["networks"] = networks
	}

	// If FlavorRef isn't provided, use FlavorName to ascertain the flavor ID.
	if opts.FlavorRef == "" {
		if opts.FlavorName == "" {
			err := servers.ErrNeitherFlavorIDNorFlavorNameProvided{}
			err.Argument = "FlavorRef/FlavorName"
			return nil, err
		}
		if sc == nil {
			err := servers.ErrNoClientProvidedForIDByName{}
			err.Argument = "ServiceClient"
			return nil, err
		}
		flavorID, err := flavors.IDFromName(sc, opts.FlavorName)
		if err != nil {
			return nil, err
		}
		b["flavorRef"] = flavorID
	}

	return map[string]interface{}{"server": b}, nil
}

// CreateHost creates a new host
// On success returns an instance of abstract.Host, and a string containing the script to execute to finalize host installation
func (s *Stack) CreateHost(request abstract.HostRequest) (host *abstract.Host, userData *userdata.Content, xerr fail.Error) {
	tracer := debug.NewTracer(nil, fmt.Sprintf("(%s)", request.ResourceName), true).WithStopwatch().GoingIn()
	defer tracer.OnExitTrace()()
	defer fail.OnExitLogError(tracer.TraceMessage(""), &xerr)()

	userData = userdata.NewContent()

	// msgFail := "failed to create Host resource: %s"
	msgSuccess := fmt.Sprintf("Host resource '%s' created successfully", request.ResourceName)

	if request.DefaultGateway == nil && !request.PublicIP {
		return nil, userData, abstract.ResourceInvalidRequestError(
			"host creation", "cannot create a host without network and without public access (would be unreachable)",
		)
	}

	// Validating name of the host
	if ok, err := validatehostName(request); !ok {
		return nil, userData, fail.Errorf(
			fmt.Sprintf(
				"name '%s' is invalid for a FlexibleEngine Host: %s", request.ResourceName,
				openstack.ProviderErrorToString(err),
			), err,
		)
	}

	// The Default Network is the first of the provided list, by convention
	defaultNetwork := request.Networks[0]
	defaultNetworkID := defaultNetwork.ID
	defaultGateway := request.DefaultGateway
	isGateway := defaultGateway == nil && defaultNetwork.Name != abstract.SingleHostNetworkName
	defaultGatewayID := ""
	defaultGatewayPrivateIP := ""
	if defaultGateway != nil {
		err := defaultGateway.Properties.LockForRead(hostproperty.NetworkV1).ThenUse(
			func(clonable data.Clonable) error {
				hostNetworkV1 := clonable.(*propsv1.HostNetwork)
				defaultGatewayPrivateIP = hostNetworkV1.IPv4Addresses[defaultNetworkID]
				defaultGatewayID = defaultGateway.ID
				return nil
			},
		)
		if err != nil {
			return nil, userData, err
		}
	}

	var nets []servers.Network
	// Add private networks
	for _, n := range request.Networks {
		nets = append(
			nets, servers.Network{
				UUID: n.ID,
			},
		)
	}

	if request.Password == "" {
		password, err := utils.GeneratePassword(16)
		if err != nil {
			return nil, userData, fail.Errorf(fmt.Sprintf("failed to generate password: %s", err.Error()), err)
		}
		request.Password = password
	}

	// --- prepares data structures for Provider usage ---

	// Constructs userdata content
	xerr = userData.Prepare(s.cfgOpts, request, defaultNetwork.CIDR, "")
	if xerr != nil {
		msg := fmt.Sprintf("failed to prepare user data content: %+v", xerr)
		logrus.Debugf(utils.Capitalize(msg))
		return nil, userData, fail.Errorf(fmt.Sprintf(msg), xerr)
	}

	// Determine system disk size based on vcpus count
	template, err := s.GetTemplate(request.TemplateID)
	if err != nil {
		return nil, userData, fail.Errorf(
			fmt.Sprintf("failed to get image: %s", openstack.ProviderErrorToString(err)), err,
		)
	}

	rim, err := s.GetImage(request.ImageID)
	if err != nil {
		return nil, userData, err
	}

	if request.DiskSize > template.DiskSize {
		template.DiskSize = request.DiskSize
	}

	if int(rim.DiskSize) > template.DiskSize {
		template.DiskSize = int(rim.DiskSize)
	}

	if template.DiskSize == 0 {
		// Determines appropriate disk size
		if template.Cores < 16 { // nolint
			template.DiskSize = 100
		} else if template.Cores < 32 {
			template.DiskSize = 200
		} else {
			template.DiskSize = 400
		}
	}

	// Select usable availability zone
	az, err := s.SelectedAvailabilityZone()
	if err != nil {
		return nil, userData, err
	}

	// Defines boot disk
	bootdiskOpts := blockDevice{
		SourceType:          exbfv.SourceImage,
		DestinationType:     exbfv.DestinationVolume,
		BootIndex:           "0",
		DeleteOnTermination: true,
		UUID:                request.ImageID,
		VolumeType:          "SSD",
		VolumeSize:          template.DiskSize,
	}
	// Defines server
	userDataPhase1, err := userData.Generate("phase1")
	if err != nil {
		return nil, userData, err
	}
	srvOpts := serverCreateOpts{
		Name:             request.ResourceName,
		SecurityGroups:   []string{s.SecurityGroup.Name},
		Networks:         nets,
		FlavorRef:        request.TemplateID,
		UserData:         userDataPhase1,
		AvailabilityZone: az,
	}
	// Defines host "Extension bootfromvolume" options
	bdOpts := bootdiskCreateOptsExt{
		CreateOptsBuilder: srvOpts,
		BlockDevice:       []blockDevice{bootdiskOpts},
	}
	b, err := bdOpts.ToServerCreateMap()
	if err != nil {
		return nil, userData, fail.Errorf(
			fmt.Sprintf(
				"failed to build query to create host '%s': %s", request.ResourceName,
				openstack.ProviderErrorToString(err),
			), err,
		)
	}

	// --- Initializes abstract.Host ---

	host = abstract.NewHost()
	host.PrivateKey = request.KeyPair.PrivateKey // Add PrivateKey to host definition
	host.Password = request.Password

	err = host.Properties.LockForWrite(hostproperty.NetworkV1).ThenUse(
		func(clonable data.Clonable) error {
			hostNetworkV1 := clonable.(*propsv1.HostNetwork)
			hostNetworkV1.IsGateway = isGateway
			hostNetworkV1.DefaultNetworkID = defaultNetworkID
			hostNetworkV1.DefaultGatewayID = defaultGatewayID
			hostNetworkV1.DefaultGatewayPrivateIP = defaultGatewayPrivateIP
			return nil
		},
	)
	if err != nil {
		return nil, userData, err
	}

	// Adds Host property SizingV1
	// template.DiskSize = diskSize // Makes sure the size of disk is correctly saved
	err = host.Properties.LockForWrite(hostproperty.SizingV1).ThenUse(
		func(clonable data.Clonable) error {
			hostSizingV1 := clonable.(*propsv1.HostSizing)
			// Note: from there, no idea what was the RequestedSize; caller will have to complement this information
			hostSizingV1.Template = request.TemplateID
			hostSizingV1.AllocatedSize = converters.ModelHostTemplateToPropertyHostSize(template)
			return nil
		},
	)
	if err != nil {
		return nil, userData, err
	}

	// --- query provider for host creation ---

	// Retry creation until success, for 10 minutes
	var (
		httpResp *http.Response
		r        servers.CreateResult
	)

	retryErr := retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			httpResp, r.Err = s.Stack.ComputeClient.Post(
				s.Stack.ComputeClient.ServiceURL("servers"), b, &r.Body, &gc.RequestOpts{
					OkCodes: []int{200, 202},
				},
			)
			server, ierr := r.Extract()
			if ierr != nil {
				if server != nil {
					servers.Delete(s.Stack.ComputeClient, server.ID)
				}
				var codeStr string
				if httpResp != nil {
					codeStr = fmt.Sprintf(" (HTTP return code: %d)", httpResp.StatusCode)
				}
				return fail.Errorf(
					fmt.Sprintf(
						"query to create host '%s' failed: %s%s",
						request.ResourceName, openstack.ProviderErrorToString(ierr), codeStr,
					), ierr,
				)
			}

			creationZone, zoneErr := s.GetAvailabilityZoneOfServer(server.ID)
			if zoneErr != nil {
				logrus.Tracef("Host successfully created but can't confirm AZ: %s", zoneErr)
			} else {
				logrus.Tracef("Host successfully created in requested AZ '%s'", creationZone)
				if creationZone != srvOpts.AvailabilityZone {
					if srvOpts.AvailabilityZone != "" {
						logrus.Warnf(
							"Host created in the WRONG availability zone: requested '%s' and got instead '%s'",
							srvOpts.AvailabilityZone, creationZone,
						)
					}
				}
			}

			host.ID = server.ID

			defer func() {
				if ierr != nil {
					servers.Delete(s.ComputeClient, server.ID)
				}
			}()

			// Wait that Host is ready, not just that the build is started
			var srv *servers.Server
			srv, ierr = s.waitHostState(host, []hoststate.Enum{hoststate.STARTED}, temporal.GetHostTimeout())
			if ierr != nil {
				return fail.Errorf(fmt.Sprintf(openstack.ProviderErrorToString(ierr)), ierr)
			}

			if ierr = s.complementHost(host, srv); ierr != nil {
				return fail.Errorf(fmt.Sprintf(openstack.ProviderErrorToString(ierr)), ierr)
			}

			return nil
		},
		temporal.GetLongOperationTimeout(),
	)
	if retryErr != nil {
		err = retryErr
		return nil, userData, err
	}
	if host == nil {
		return nil, userData, fail.Errorf(fmt.Sprintf("unexpected problem creating host"), nil)
	}

	newHost := host
	// Starting from here, delete host if exiting with error
	defer func() {
		if err != nil {
			derr := s.DeleteHost(newHost.ID)
			if derr != nil {
				switch derr.(type) {
				case fail.ErrNotFound:
					logrus.Errorf(
						"Cleaning up on failure, failed to delete host '%s', resource not found: '%v'", newHost.Name,
						derr,
					)
				case fail.ErrTimeout:
					logrus.Errorf(
						"Cleaning up on failure, failed to delete host '%s', timeout: '%v'", newHost.Name, derr,
					)
				default:
					logrus.Errorf("Cleaning up on failure, failed to delete host '%s': '%v'", newHost.Name, derr)
				}
				err = fail.AddConsequence(err, derr)
			}
		}
	}()

	if request.PublicIP {
		var fip *FloatingIP
		fip, err = s.attachFloatingIP(host)
		if err != nil {
			return nil, userData, fail.Errorf(
				fmt.Sprintf(
					"error attaching public IP for host '%s': %s", request.ResourceName,
					openstack.ProviderErrorToString(err),
				), err,
			)
		}
		if fip == nil {
			return nil, userData, fail.Errorf(fmt.Sprintf("error attaching public IP for host: unknown error"), nil)
		}

		// Starting from here, delete Floating IP if exiting with error
		defer func() {
			if err != nil {
				derr := s.DeleteFloatingIP(fip.ID)
				if derr != nil {
					logrus.Errorf("Error deleting Floating IP: %v", derr)
					err = fail.AddConsequence(err, derr)
				}
			}
		}()

		// Updates Host property NetworkV1 in host instance
		err = host.Properties.LockForWrite(hostproperty.NetworkV1).ThenUse(
			func(clonable data.Clonable) error {
				hostNetworkV1 := clonable.(*propsv1.HostNetwork)
				if ipversion.IPv4.Is(fip.PublicIPAddress) {
					hostNetworkV1.PublicIPv4 = fip.PublicIPAddress
				} else if ipversion.IPv6.Is(fip.PublicIPAddress) {
					hostNetworkV1.PublicIPv6 = fip.PublicIPAddress
				}
				userData.PublicIP = fip.PublicIPAddress
				return nil
			},
		)
		if err != nil {
			return nil, userData, err
		}

		if defaultGateway == nil && defaultNetwork.Name != abstract.SingleHostNetworkName {
			err = s.enableHostRouterMode(host)
			if err != nil {
				return nil, userData, fail.Errorf(
					fmt.Sprintf(
						"error enabling gateway mode of host '%s': %s", request.ResourceName,
						openstack.ProviderErrorToString(err),
					), err,
				)
			}
		}
	}

	logrus.Infoln(msgSuccess)
	return host, userData, nil
}

// validatehostName validates the name of an host based on known FlexibleEngine requirements
func validatehostName(req abstract.HostRequest) (bool, fail.Error) {
	s := check.Struct{
		"ResourceName": check.Composite{
			check.NonEmpty{},
			check.Regex{Constraint: `^[a-zA-Z0-9_-]+$`},
			check.MaxChar{Constraint: 64},
		},
	}

	e := s.Validate(req)
	if e.HasErrors() {
		errorList, _ := e.GetErrorsByKey("ResourceName")
		var errs []error
		for _, msg := range errorList {
			errs = append(errs, fmt.Errorf(msg.Error()))
		}

		return false, fail.Errorf("failure validating host name", fail.ErrListError(errs))
	}
	return true, nil
}

// InspectHost updates the data inside host with the data from provider
func (s *Stack) InspectHost(hostParam interface{}) (host *abstract.Host, xerr fail.Error) {
	switch hostParam := hostParam.(type) {
	case string:
		if hostParam == "" {
			return nil, fail.InvalidParameterError("hostParam", "cannot be an empty string")
		}
		host = abstract.NewHost()
		host.ID = hostParam
	case *abstract.Host:
		if hostParam == nil {
			return nil, fail.InvalidParameterError("hostParam", "cannot be nil")
		}
		host = hostParam
	default:
		return nil, fail.InvalidParameterError("hostParam", "must be a string or a *abstract.Host")
	}

	serverState, err := s.GetHostState(host.ID)
	if err != nil {
		return nil, err
	}

	switch serverState {
	case hoststate.STARTED, hoststate.STOPPED:
		server, err := s.waitHostState(
			host.ID, []hoststate.Enum{hoststate.STARTED, hoststate.STOPPED}, 2*temporal.GetBigDelay(),
		)
		if err != nil {
			return nil, err
		}

		err = s.complementHost(host, server)
		if err != nil {
			return nil, err
		}

		if !host.OK() {
			logrus.Warnf("[TRACE] Unexpected host status: %s", spew.Sdump(host))
		}
	default:
		host.LastState = serverState
	}

	return host, err
}

// complementHost complements Host data with content of server parameter
func (s *Stack) complementHost(host *abstract.Host, server *servers.Server) error {
	networks, addresses, ipv4, ipv6, err := s.collectAddresses(host)
	if err != nil {
		return err
	}

	// Updates intrinsic data of host if needed
	if host.ID == "" {
		host.ID = server.ID
	}
	if host.Name == "" {
		host.Name = server.Name
	}

	host.LastState = toHostState(server.Status)
	// VPL: I don't get the point of this...
	// switch host.LastState {
	// case hoststate.STARTED, hoststate.STOPPED:
	//	// continue
	// default:
	//	logrus.Warnf("[TRACE] Unexpected host's last state: %v", host.LastState)
	// }
	// ENDVPL

	// Updates Host Property propsv1.HostDescription
	err = host.Properties.LockForWrite(hostproperty.DescriptionV1).ThenUse(
		func(clonable data.Clonable) error {
			hostDescriptionV1 := clonable.(*propsv1.HostDescription)
			hostDescriptionV1.Created = server.Created
			hostDescriptionV1.Updated = server.Updated
			return nil
		},
	)
	if err != nil {
		return err
	}

	// Updates Host Property HostNetwork
	return host.Properties.LockForWrite(hostproperty.NetworkV1).ThenUse(
		func(clonable data.Clonable) error {
			hostNetworkV1 := clonable.(*propsv1.HostNetwork)
			if hostNetworkV1.PublicIPv4 == "" {
				hostNetworkV1.PublicIPv4 = ipv4
			}
			if hostNetworkV1.PublicIPv6 == "" {
				hostNetworkV1.PublicIPv6 = ipv6
			}

			if len(hostNetworkV1.NetworksByID) > 0 {
				ipv4Addresses := map[string]string{}
				ipv6Addresses := map[string]string{}
				for netid, netname := range hostNetworkV1.NetworksByID {
					if ip, ok := addresses[ipversion.IPv4][netid]; ok {
						ipv4Addresses[netid] = ip
					} else if ip, ok := addresses[ipversion.IPv4][netname]; ok {
						ipv4Addresses[netid] = ip
					} else {
						ipv4Addresses[netid] = ""
					}

					if ip, ok := addresses[ipversion.IPv6][netid]; ok {
						ipv6Addresses[netid] = ip
					} else if ip, ok := addresses[ipversion.IPv6][netname]; ok {
						ipv6Addresses[netid] = ip
					} else {
						ipv6Addresses[netid] = ""
					}
				}
				hostNetworkV1.IPv4Addresses = ipv4Addresses
				hostNetworkV1.IPv6Addresses = ipv6Addresses
			} else {
				networksByID := map[string]string{}
				ipv4Addresses := map[string]string{}
				ipv6Addresses := map[string]string{}
				for _, netid := range networks {
					networksByID[netid] = ""

					if ip, ok := addresses[ipversion.IPv4][netid]; ok {
						ipv4Addresses[netid] = ip
					} else {
						ipv4Addresses[netid] = ""
					}

					if ip, ok := addresses[ipversion.IPv6][netid]; ok {
						ipv6Addresses[netid] = ip
					} else {
						ipv6Addresses[netid] = ""
					}
				}
				hostNetworkV1.NetworksByID = networksByID
				// IPvxAddresses are here indexed by names... At least we have them...
				hostNetworkV1.IPv4Addresses = ipv4Addresses
				hostNetworkV1.IPv6Addresses = ipv6Addresses
			}

			// Updates network name and relationships if needed
			for netid, netname := range hostNetworkV1.NetworksByID {
				if netname == "" {
					network, err := s.GetNetwork(netid)
					if err != nil {
						logrus.Errorf("failed to get network '%s'", netid)
						continue
					}
					hostNetworkV1.NetworksByID[netid] = network.Name
					hostNetworkV1.NetworksByName[network.Name] = netid
				}
			}
			return nil
		},
	)
}

// collectAddresses converts adresses returned by the OpenStack driver
// Returns string slice containing the name of the networks, string map of IP addresses
// (indexed on network name), public ipv4 and ipv6 (if they exists)
func (s *Stack) collectAddresses(host *abstract.Host) ([]string, map[ipversion.Enum]map[string]string, string, string, fail.Error) {
	var (
		networks      []string
		addrs         = map[ipversion.Enum]map[string]string{}
		AcccessIPv4   string
		AcccessIPv6   string
		allInterfaces []nics.Interface
	)

	pager := s.listInterfaces(host.ID)
	err := pager.EachPage(
		func(page pagination.Page) (bool, fail.Error) {
			list, err := nics.ExtractInterfaces(page)
			if err != nil {
				return false, err
			}
			allInterfaces = append(allInterfaces, list...)
			return true, nil
		},
	)
	if err != nil {
		return networks, addrs, "", "", err
	}

	addrs[ipversion.IPv4] = map[string]string{}
	addrs[ipversion.IPv6] = map[string]string{}

	for _, item := range allInterfaces {
		networks = append(networks, item.NetID)
		for _, address := range item.FixedIPs {
			fixedIP := address.IPAddress
			ipv4 := net.ParseIP(fixedIP).To4() != nil
			if item.NetID == s.cfgOpts.ProviderNetwork {
				if ipv4 {
					AcccessIPv4 = fixedIP
				} else {
					AcccessIPv6 = fixedIP
				}
			} else {
				if ipv4 {
					addrs[ipversion.IPv4][item.NetID] = fixedIP
				} else {
					addrs[ipversion.IPv6][item.NetID] = fixedIP
				}
			}
		}
	}
	return networks, addrs, AcccessIPv4, AcccessIPv6, nil
}

// ListHosts lists available hosts
func (s *Stack) ListHosts() ([]*abstract.Host, fail.Error) {
	pager := servers.List(s.Stack.ComputeClient, servers.ListOpts{})
	var hosts []*abstract.Host
	err := pager.EachPage(
		func(page pagination.Page) (bool, fail.Error) {
			list, err := servers.ExtractServers(page)
			if err != nil {
				return false, err
			}

			for _, srv := range list {
				h := abstract.NewHost()
				h.ID = srv.ID
				err := s.complementHost(h, &srv)
				if err != nil {
					return false, err
				}
				hosts = append(hosts, h)
			}
			return true, nil
		},
	)
	if len(hosts) == 0 && err != nil {
		return nil, fail.Errorf(fmt.Sprintf("error listing hosts: %s", openstack.ProviderErrorToString(err)), err)
	}
	return hosts, nil
}

// DeleteHost deletes the host identified by id
func (s *Stack) DeleteHost(id string) error {
	// Delete floating IP address if there is one
	if s.cfgOpts.UseFloatingIP {
		fip, err := s.getFloatingIPOfHost(id)
		if err != nil {
			switch err.(type) {
			case fail.ErrNotFound:
				// Continue
			default:
				return fail.Wrap(err, fmt.Sprintf("error retrieving floating ip for '%s'", id))
			}
		} else if fip != nil {
			err = floatingips.DisassociateInstance(
				s.Stack.ComputeClient, id, floatingips.DisassociateOpts{FloatingIP: fip.IP},
			).ExtractErr()
			if err != nil {
				return fail.Errorf(
					fmt.Sprintf(
						"error deleting host %s : %s", id, openstack.ProviderErrorToString(err),
					), err,
				)
			}
			err = floatingips.Delete(s.Stack.ComputeClient, fip.ID).ExtractErr()
			if err != nil {
				return fail.Errorf(
					fmt.Sprintf(
						"error deleting host %s : %s", id, openstack.ProviderErrorToString(err),
					), err,
				)
			}
		}
	}

	// Try to remove host for 3 minutes
	retryErr := retry.WhileUnsuccessful(
		func() error {
			resourcePresent := true
			// 1st, send delete host order
			innerErr := servers.Delete(s.Stack.ComputeClient, id).ExtractErr()
			if innerErr != nil {
				return openstack.ReinterpretGophercloudErrorCode(
					innerErr, []int64{404}, []int64{408, 429, 500, 503}, []int64{409}, func(ferr error) error {
						return fail.AbortedError("", ferr)
					},
				)
			}
			// 2nd, check host status every 5 seconds until check failed.
			// If check succeeds but state is Error, retry the deletion.
			// If check fails and error isn't 'resource not found', retry
			var host *servers.Server
			innerRetryErr := retry.WhileUnsuccessfulDelay5Seconds(
				func() error {
					host, innerErr = servers.Get(s.Stack.ComputeClient, id).Extract()
					if innerErr == nil {
						if toHostState(host.Status) == hoststate.ERROR {
							return nil
						}
						return fail.Errorf(fmt.Sprintf("host '%s' state is '%s'", host.Name, host.Status), innerErr)
					}

					if innerErr != nil {
						rerr := openstack.ReinterpretGophercloudErrorCode(
							innerErr, []int64{404}, []int64{408, 429, 500, 503}, []int64{409}, func(ferr error) error {
								return fail.AbortedError("", ferr)
							},
						)
						if rerr == nil {
							resourcePresent = false
						}
						return rerr
					}

					return innerErr
				},
				temporal.GetContextTimeout(),
			)
			if innerRetryErr != nil {
				if _, ok := innerRetryErr.(retry.ErrTimeout); ok {
					// retry deletion...
					return abstract.TimeoutError(
						fmt.Sprintf(
							"host '%s' not deleted after %v", id, temporal.GetContextTimeout(),
						), temporal.GetContextTimeout(),
					)
				}
				return innerRetryErr
			}
			if !resourcePresent {
				return nil
			}
			return fail.Errorf(fmt.Sprintf("host '%s' in state 'ERROR', retrying to delete", id), nil)
		},
		5*time.Second,
		temporal.GetHostCleanupTimeout(),
	)
	if retryErr != nil {
		logrus.Errorf("failed to remove host '%s': %s", id, retryErr.Error())
		return retryErr
	}
	return nil
}

// getFloatingIP returns the floating IP associated with the host identified by hostID
// By convention only one floating IP is allocated to an host
func (s *Stack) getFloatingIPOfHost(hostID string) (*floatingips.FloatingIP, fail.Error) {
	pager := floatingips.List(s.Stack.ComputeClient)
	var fips []floatingips.FloatingIP
	retryErr := pager.EachPage(
		func(page pagination.Page) (bool, fail.Error) {
			list, err := floatingips.ExtractFloatingIPs(page)
			if err != nil {
				return false, err
			}

			for _, fip := range list {
				if fip.InstanceID == hostID {
					fips = append(fips, fip)
				}
			}
			return true, nil
		},
	)
	if len(fips) == 0 {
		if retryErr != nil {
			return nil, fail.NotFoundError(
				fmt.Sprintf(
					"no floating IP found for host '%s': %s", hostID, openstack.ProviderErrorToString(retryErr),
				),
			)
		}
		return nil, fail.NotFoundError(fmt.Sprintf("no floating IP found for host '%s'", hostID))
	}
	if len(fips) > 1 {
		return nil, fail.Errorf(
			fmt.Sprintf(
				"configuration error, more than one Floating IP associated to host '%s'", hostID,
			), nil,
		)
	}
	return &fips[0], nil
}

// attachFloatingIP creates a Floating IP and attaches it to an host
func (s *Stack) attachFloatingIP(host *abstract.Host) (*FloatingIP, fail.Error) {
	fip, err := s.CreateFloatingIP()
	if err != nil {
		return nil, fail.Errorf(
			fmt.Sprintf(
				"failed to attach Floating IP on host '%s': %s", host.Name, openstack.ProviderErrorToString(err),
			), nil,
		)
	}

	err = s.AssociateFloatingIP(host, fip.ID)
	if err != nil {
		derr := s.DeleteFloatingIP(fip.ID)
		if derr != nil {
			logrus.Errorf("Error deleting Floating IP: %v", derr)
			err = fail.AddConsequence(err, derr)
		}

		return nil, err
	}
	return fip, nil
}

// EnableHostRouterMode enables the host to act as a router/gateway.
func (s *Stack) enableHostRouterMode(host *abstract.Host) error {
	var (
		portID *string
		err    error
	)

	// Sometimes, getOpenstackPortID doesn't find network interface, so let's retry in case it's a bad timing issue
	retryErr := retry.WhileUnsuccessfulDelay5SecondsTimeout(
		func() error {
			portID, err = s.getOpenstackPortID(host)
			if err != nil {
				return fail.Errorf(fmt.Sprintf("%s", openstack.ProviderErrorToString(err)), err)
			}
			if portID == nil {
				return fail.Errorf(fmt.Sprintf("failed to find OpenStack port"), nil)
			}
			return nil
		},
		temporal.GetBigDelay(),
	)
	if retryErr != nil {
		return fail.Errorf(fmt.Sprintf("failed to enable Router Mode on host '%s': %v", host.Name, retryErr), retryErr)
	}

	pairs := []ports.AddressPair{
		{
			IPAddress: "1.1.1.1/0",
		},
	}
	opts := ports.UpdateOpts{AllowedAddressPairs: &pairs}
	_, err = ports.Update(s.Stack.NetworkClient, *portID, opts).Extract()
	if err != nil {
		return fail.Errorf(
			fmt.Sprintf(
				"failed to enable Router Mode on host '%s': %s", host.Name, openstack.ProviderErrorToString(err),
			), err,
		)
	}
	return nil
}

// DisableHostRouterMode disables the host to act as a router/gateway.
func (s *Stack) disableHostRouterMode(host *abstract.Host) error {
	portID, err := s.getOpenstackPortID(host)
	if err != nil {
		return fail.Errorf(
			fmt.Sprintf(
				"failed to disable Router Mode on host '%s': %s", host.Name, openstack.ProviderErrorToString(err),
			), err,
		)
	}
	if portID == nil {
		return fail.Errorf(
			fmt.Sprintf(
				"failed to disable Router Mode on host '%s': failed to find OpenStack port", host.Name,
			), nil,
		)
	}

	opts := ports.UpdateOpts{AllowedAddressPairs: nil}
	_, err = ports.Update(s.Stack.NetworkClient, *portID, opts).Extract()
	if err != nil {
		return fail.Errorf(
			fmt.Sprintf(
				"failed to disable Router Mode on host '%s': %s", host.Name, openstack.ProviderErrorToString(err),
			), err,
		)
	}
	return nil
}

// listInterfaces returns a pager of the interfaces attached to host identified by 'serverID'
func (s *Stack) listInterfaces(hostID string) pagination.Pager {
	url := s.Stack.ComputeClient.ServiceURL("servers", hostID, "os-interface")
	return pagination.NewPager(
		s.Stack.ComputeClient, url, func(r pagination.PageResult) pagination.Page {
			return nics.InterfacePage{SinglePageBase: pagination.SinglePageBase(r)}
		},
	)
}

// getOpenstackPortID returns the port ID corresponding to the first private IP address of the host
// returns nil,nil if not found
func (s *Stack) getOpenstackPortID(host *abstract.Host) (*string, fail.Error) {
	ip := host.GetPrivateIP()
	found := false
	nic := nics.Interface{}
	pager := s.listInterfaces(host.ID)
	err := pager.EachPage(
		func(page pagination.Page) (bool, fail.Error) {
			list, err := nics.ExtractInterfaces(page)
			if err != nil {
				return false, err
			}
			for _, i := range list {
				for _, iip := range i.FixedIPs {
					if iip.IPAddress == ip {
						found = true
						nic = i
						return false, nil
					}
				}
			}
			return true, nil
		},
	)
	if err != nil {
		return nil, fail.Errorf(
			fmt.Sprintf(
				"error browsing Openstack Interfaces of host '%s': %s", host.Name, openstack.ProviderErrorToString(err),
			), err,
		)
	}
	if found {
		return &nic.PortID, nil
	}
	return nil, abstract.ResourceNotFoundError("Port ID corresponding to host", host.Name)
}

// toHostSize converts flavor attributes returned by OpenStack driver into abstract.hostproperty.v1.HostSize
func (s *Stack) toHostSize(flavor map[string]interface{}) *propsv1.HostSize {
	if i, ok := flavor["id"]; ok {
		fid := i.(string)
		tpl, _ := s.GetTemplate(fid)
		return converters.ModelHostTemplateToPropertyHostSize(tpl)
	}
	hostSize := propsv1.NewHostSize()
	if _, ok := flavor["vcpus"]; ok {
		hostSize.Cores = flavor["vcpus"].(int)
		hostSize.DiskSize = flavor["disk"].(int)
		hostSize.RAMSize = flavor["ram"].(float32) / 1000.0
	}
	return hostSize
}

// toHostState converts host status returned by FlexibleEngine driver into HostState enum
func toHostState(status string) hoststate.Enum {
	switch status {
	case "BUILD", "build", "BUILDING", "building":
		return hoststate.STARTING
	case "ACTIVE", "active":
		return hoststate.STARTED
	case "RESCUED", "rescued":
		return hoststate.STOPPING
	case "STOPPED", "stopped", "SHUTOFF", "shutoff":
		return hoststate.STOPPED
	default:
		return hoststate.ERROR
	}
}

// waitHostState waits an host achieve ready state
// hostParam can be an ID of host, or an instance of *abstract.Host; any other type will return an utils.ErrInvalidParameter
func (s *Stack) waitHostState(hostParam interface{}, states []hoststate.Enum, timeout time.Duration) (server *servers.Server, xerr fail.Error) {
	var host *abstract.Host

	switch hostParam := hostParam.(type) {
	case string:
		host = abstract.NewHost()
		host.ID = hostParam
	case *abstract.Host:
		host = hostParam
	}
	if host == nil {
		return nil, fail.InvalidParameterError("hostParam", "must be a not-empty string or a *abstract.Host!")
	}

	hostRef := host.Name
	if hostRef == "" {
		hostRef = host.ID
	}

	defer debug.NewTracer(nil, fmt.Sprintf("(%s)", hostRef), true).WithStopwatch().GoingIn().OnExitTrace()()

	retryErr := retry.WhileUnsuccessful(
		func() error {
			server, xerr = servers.Get(s.ComputeClient, host.ID).Extract()
			if xerr != nil {
				return openstack.ReinterpretGophercloudErrorCode(
					xerr, nil, []int64{408, 429, 500, 503}, []int64{404, 409}, func(ferr error) error {
						return fail.AbortedError("", ferr)
					},
				)
			}

			if server == nil {
				return fail.Errorf(fmt.Sprintf("error getting host, nil response from gophercloud"), nil)
			}

			lastState := toHostState(server.Status)
			// If state matches, we consider this a success no matter what
			for _, state := range states {
				if lastState == state {
					return nil
				}
			}

			// logrus.Warnf("Target state: %s, current state: %s", states, lastState)

			if lastState == hoststate.ERROR {
				return fail.AbortedError("", abstract.ResourceNotAvailableError("host", host.ID))
			}

			if !((lastState == hoststate.STARTING) || (lastState == hoststate.STOPPING)) {
				return fail.Errorf(
					fmt.Sprintf(
						"host status of '%s' is in state '%s', and that's not a transition state", host.ID,
						server.Status,
					), nil,
				)
			}

			return fail.Errorf(fmt.Sprintf("server not ready yet"), nil)
		},
		temporal.GetMinDelay(),
		timeout,
	)
	if retryErr != nil {
		if _, ok := retryErr.(retry.ErrTimeout); ok {
			return nil, abstract.TimeoutError(
				fmt.Sprintf(
					"timeout waiting to get host '%s' information after %v", host.Name, timeout,
				), timeout,
			)
		}

		if aborted, ok := retryErr.(retry.ErrAborted); ok {
			return nil, aborted.Cause()
		}

		return nil, retryErr
	}

	return server, nil
}
