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

package gcp

import (
	"fmt"
	"strconv"
	"time"

	"github.com/CS-SI/SafeScale/lib/utils/debug"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"

	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract"
	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/enums/hostproperty"
	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/enums/hoststate"
	converters "github.com/CS-SI/SafeScale/lib/server/iaas/abstract/properties"
	propsv1 "github.com/CS-SI/SafeScale/lib/server/iaas/abstract/properties/v1"
	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/userdata"
	"github.com/CS-SI/SafeScale/lib/utils"
	"github.com/CS-SI/SafeScale/lib/utils/data"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
	"github.com/CS-SI/SafeScale/lib/utils/retry"
	"github.com/CS-SI/SafeScale/lib/utils/temporal"
)

// -------------IMAGES---------------------------------------------------------------------------------------------------

// ListImages lists available OS images
func (s *Stack) ListImages() (images []abstract.Image, xerr fail.Error) {
	compuService := s.ComputeService

	images = []abstract.Image{}

	families := []string{
		"centos-cloud", "debian-cloud", "rhel-cloud", "ubuntu-os-cloud", "suse-cloud", "rhel-sap-cloud",
		"suse-sap-cloud",
	}

	for _, family := range families {
		token := ""
		for paginate := true; paginate; {
			resp, err := compuService.Images.List(family).Filter("deprecated.replacement ne .*images.*").PageToken(token).Do()
			if err != nil {
				logrus.Warnf("Can't list public images for project %q", family)
				break
			}

			for _, image := range resp.Items {
				images = append(
					images, abstract.Image{Name: image.Name, URL: image.SelfLink, ID: strconv.FormatUint(image.Id, 10), DiskSize: image.DiskSizeGb},
				)
			}
			token := resp.NextPageToken
			paginate = token != ""
		}
	}

	if len(images) == 0 {
		return images, xerr
	}

	return images, nil
}

// GetImage returns the Image referenced by id
func (s *Stack) GetImage(id string) (*abstract.Image, fail.Error) {
	images, err := s.ListImages()
	if err != nil {
		return nil, err
	}

	for _, image := range images {
		if image.ID == id {
			return &image, nil
		}
	}

	return nil, fail.Errorf(fmt.Sprintf("image with id [%s] not found", id), nil)
}

// -------------TEMPLATES------------------------------------------------------------------------------------------------

// ListTemplates overload OpenStackGcp ListTemplate method to filter wind and flex instance and add GPU configuration
func (s *Stack) ListTemplates(all bool) (templates []abstract.HostTemplate, xerr fail.Error) {
	compuService := s.ComputeService

	templates = []abstract.HostTemplate{}

	token := ""
	for paginate := true; paginate; {
		resp, err := compuService.MachineTypes.List(s.GcpConfig.ProjectID, s.GcpConfig.Zone).PageToken(token).Do()
		if err != nil {
			logrus.Warnf("Can't list public types...: %s", err)
			break
		} else {

			for _, matype := range resp.Items {
				ht := abstract.HostTemplate{
					Cores:    int(matype.GuestCpus),
					RAMSize:  float32(matype.MemoryMb / 1024),
					DiskSize: int(matype.ImageSpaceGb),
					ID:       strconv.FormatUint(matype.Id, 10),
					Name:     matype.Name,
				}
				templates = append(templates, ht)
			}
		}
		token := resp.NextPageToken
		paginate = token != ""
	}

	if len(templates) == 0 {
		return templates, xerr
	}

	return templates, nil
}

// GetTemplate overload OpenStackGcp GetTemplate method to add GPU configuration
func (s *Stack) GetTemplate(id string) (*abstract.HostTemplate, fail.Error) {
	templates, err := s.ListTemplates(true)
	if err != nil {
		return nil, err
	}

	for _, template := range templates {
		if template.ID == id {
			return &template, nil
		}
	}

	return nil, fail.Errorf(fmt.Sprintf("template with id [%s] not found", id), nil)
}

// -------------SSH KEYS-------------------------------------------------------------------------------------------------

// CreateKeyPair creates a key pair (no import)
func (s *Stack) CreateKeyPair(name string) (*abstract.KeyPair, fail.Error) {
	return abstract.NewKeyPair(name)
}

// GetKeyPair returns the key pair identified by id
func (s *Stack) GetKeyPair(id string) (*abstract.KeyPair, fail.Error) {
	return nil, fail.NotImplementedError("GetKeyPair() not implemented yet") // FIXME: Technical debt
}

// ListKeyPairs lists available key pairs
func (s *Stack) ListKeyPairs() ([]abstract.KeyPair, fail.Error) {
	return nil, fail.NotImplementedError("ListKeyPairs() not implemented yet") // FIXME: Technical debt
}

// DeleteKeyPair deletes the key pair identified by id
func (s *Stack) DeleteKeyPair(id string) error {
	return nil
}

// CreateHost creates an host satisfying request
func (s *Stack) CreateHost(request abstract.HostRequest) (host *abstract.Host, userData *userdata.Content, xerr fail.Error) {
	userData = userdata.NewContent()

	resourceName := request.ResourceName
	networks := request.Networks
	hostMustHavePublicIP := request.PublicIP

	if len(networks) == 0 {
		return nil, userData, fail.Errorf(
			fmt.Sprintf(
				"the host %s must be on at least one network (even if public)", resourceName,
			), nil,
		)
	}

	if request.Password == "" {
		password, err := utils.GeneratePassword(16)
		if err != nil {
			return nil, userData, fail.Errorf(fmt.Sprintf("failed to generate password: %s", err.Error()), err)
		}
		request.Password = password
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

	if defaultGateway == nil && !hostMustHavePublicIP {
		return nil, userData, fail.Errorf(
			fmt.Sprintf("the host %s must have a gateway or be public", resourceName), nil,
		)
	}

	// --- prepares data structures for Provider usage ---

	// Constructs userdata content
	xerr = userData.Prepare(*s.Config, request, defaultNetwork.CIDR, "")
	if xerr != nil {
		msg := fmt.Sprintf("failed to prepare user data content: %+v", xerr)
		logrus.Debugf(utils.Capitalize(msg))
		return nil, userData, fail.Errorf(fmt.Sprintf(msg), xerr)
	}

	// Determine system disk size based on vcpus count
	template, err := s.GetTemplate(request.TemplateID)
	if err != nil {
		return nil, userData, fail.Errorf(fmt.Sprintf("failed to get image: %s", err), err)
	}

	rim, err := s.GetImage(request.ImageID)
	if err != nil {
		return nil, nil, err
	}

	// select disk size

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

	logrus.Debugf("Selected template: '%s', '%s', '%d Gb'", template.ID, template.Name, template.DiskSize)

	// Select usable availability zone, the first one in the list
	if s.GcpConfig.Zone == "" {
		azList, err := s.ListAvailabilityZones()
		if err != nil {
			return nil, userData, err
		}
		var az string
		for az = range azList {
			break
		}
		s.GcpConfig.Zone = az
		logrus.Debugf("Selected Availability Zone: '%s'", az)
	}

	// Sets provider parameters to create host
	userDataPhase1, err := userData.Generate("phase1")
	if err != nil {
		return nil, userData, err
	}

	// --- Initializes abstract.Host ---

	host = abstract.NewHost()
	host.PrivateKey = request.KeyPair.PrivateKey // Add PrivateKey to host definition
	host.Password = request.Password

	err = host.Properties.LockForWrite(hostproperty.NetworkV1).ThenUse(
		func(clonable data.Clonable) error {
			hostNetworkV1 := clonable.(*propsv1.HostNetwork)
			hostNetworkV1.DefaultNetworkID = defaultNetworkID
			hostNetworkV1.DefaultGatewayID = defaultGatewayID
			hostNetworkV1.DefaultGatewayPrivateIP = defaultGatewayPrivateIP
			hostNetworkV1.IsGateway = isGateway
			return nil
		},
	)
	if err != nil {
		return nil, userData, err
	}

	// Adds Host property SizingV1
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

	logrus.Debugf("requesting host resource creation...")
	var desistError error

	// Retry creation until success, for 10 minutes
	retryErr := retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			server, err := buildGcpMachine(
				s.ComputeService, s.GcpConfig.ProjectID, request.ResourceName, rim.URL, s.GcpConfig.Region,
				s.GcpConfig.Zone, s.GcpConfig.NetworkName, defaultNetwork.Name, string(userDataPhase1), isGateway,
				template,
			)
			if err != nil {
				if server != nil {
					// try deleting server
					killErr := s.DeleteHost(server.ID)
					if killErr != nil {
						switch killErr.(type) {
						case fail.ErrTimeout:
							logrus.Error("Timeout cleaning up gcp instance")
						default:
							logrus.Errorf("Something else happened to gcp instance: %+v", killErr)
						}
						err = fail.AddConsequence(err, killErr)
					}
					return err
				}

				if gerr, ok := err.(*googleapi.Error); ok {
					logrus.Warnf("Received GCP errorcode: %d", gerr.Code)

					if !(gerr.Code == 200 || gerr.Code == 429 || gerr.Code == 500 || gerr.Code == 503) {
						desistError = gerr
						return nil
					}
				}

				logrus.Warnf("error creating host: %+v", err)
				return err
			}

			if server == nil {
				return fail.Errorf(fmt.Sprintf("failed to create server"), nil)
			}

			host.ID = server.ID
			host.Name = server.Name

			// Wait that Host is ready, not just that the build is started
			_, err = s.WaitHostReady(host, temporal.GetLongOperationTimeout())
			if err != nil {
				killErr := s.DeleteHost(host.ID)
				if killErr != nil {
					switch killErr.(type) {
					case fail.ErrTimeout:
						logrus.Error("Timeout cleaning up gcp instance")
					default:
						logrus.Errorf("Something else happened to gcp instance: %+v", killErr)
					}
					err = fail.AddConsequence(err, killErr)
				}
				return err
			}
			return nil
		},
		temporal.GetLongOperationTimeout(),
	)
	if retryErr != nil {
		return nil, userData, retryErr
	}
	if desistError != nil {
		return nil, userData, abstract.ResourceForbiddenError(
			request.ResourceName, fmt.Sprintf("error creating host: %s", desistError.Error()),
		)
	}

	logrus.Debugf("host resource created.")

	newHost := host
	// Starting from here, delete host if exiting with error
	defer func() {
		if err != nil {
			logrus.Infof("Cleanup, deleting host '%s'", newHost.Name)
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

	if host == nil {
		return nil, nil, fail.Errorf(fmt.Sprintf("unexpected nil host"), nil)
	}

	if !host.OK() {
		logrus.Warnf("Missing data in host: %s", spew.Sdump(host))
	}

	return host, userData, nil
}

// WaitHostReady waits an host achieve ready state
// hostParam can be an ID of host, or an instance of *abstract.Host; any other type will return an utils.ErrInvalidParameter.
func (s *Stack) WaitHostReady(hostParam interface{}, timeout time.Duration) (res *abstract.Host, xerr fail.Error) {
	var host *abstract.Host
	switch hostParam := hostParam.(type) {
	case string:
		host = abstract.NewHost()
		host.ID = hostParam
	case *abstract.Host:
		host = hostParam
	}
	if host == nil {
		return nil, fail.InvalidParameterError("hostParam", "must be a not-empty string or a *abstract.Host")
	}

	tracer := debug.NewTracer(nil, fmt.Sprintf("(%s)", host.ID), true).GoingIn()
	defer tracer.OnExitTrace()()
	defer fail.OnExitLogError(tracer.TraceMessage(""), &xerr)()

	retryErr := retry.WhileUnsuccessful(
		func() error {
			hostTmp, err := s.InspectHost(host)
			if err != nil {
				return err
			}

			host = hostTmp
			if host.LastState != hoststate.STARTED {
				return fail.Errorf(fmt.Sprintf("not in ready state (current state: %s)", host.LastState.String()), nil)
			}
			return nil
		},
		temporal.GetDefaultDelay(),
		timeout,
	)
	if retryErr != nil {
		if _, ok := retryErr.(retry.ErrTimeout); ok {
			return host, abstract.TimeoutError(
				fmt.Sprintf(
					"timeout waiting to get host '%s' information after %v", host.Name, timeout,
				), timeout,
			)
		}
		return host, retryErr
	}
	return host, nil
}

func publicAccess(isPublic bool) []*compute.AccessConfig {
	if isPublic {
		return []*compute.AccessConfig{
			{
				Type: "ONE_TO_ONE_NAT",
				Name: "External NAT",
			},
		}
	}

	return []*compute.AccessConfig{}
}

// buildGcpMachine ...
func buildGcpMachine(service *compute.Service, projectID string, instanceName string, imageID string, region string, zone string, network string, subnetwork string, userdata string, isPublic bool, template *abstract.HostTemplate) (*abstract.Host, fail.Error) {
	prefix := "https://www.googleapis.com/compute/v1/projects/" + projectID

	imageURL := imageID

	tag := "nat"
	if !isPublic {
		tag = fmt.Sprintf("no-ip-%s", subnetwork)
	}

	// logrus.Warnf("Receiving a disk request of %d", template.DiskSize)

	instance := &compute.Instance{
		Name:         instanceName,
		Description:  "compute sample instance",
		MachineType:  prefix + "/zones/" + zone + "/machineTypes/" + template.Name,
		CanIpForward: isPublic,
		Tags: &compute.Tags{
			Items: []string{tag},
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    fmt.Sprintf("%s-disk", instanceName),
					SourceImage: imageURL,
					DiskSizeGb:  int64(template.DiskSize),
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: publicAccess(isPublic),
				Network:       prefix + "/global/networks/" + network,
				Subnetwork:    prefix + "/regions/" + region + "/subnetworks/" + subnetwork,
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.DevstorageFullControlScope,
					compute.ComputeScope,
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "startup-script",
					Value: &userdata,
				},
			},
		},
	}

	op, err := service.Instances.Insert(projectID, zone, instance).Do()
	if err != nil {
		return nil, err
	}

	etag := op.Header.Get("Etag")
	oco := OpContext{
		Operation:    op,
		ProjectID:    projectID,
		Service:      service,
		DesiredState: "DONE",
	}

	err = waitUntilOperationIsSuccessfulOrTimeout(oco, temporal.GetMinDelay(), temporal.GetHostTimeout())
	if err != nil {
		return nil, err
	}

	inst, err := service.Instances.Get(projectID, zone, instanceName).IfNoneMatch(etag).Do()
	if err != nil {
		return nil, err
	}

	logrus.Tracef("Got compute.Instance, err: %#v, %v", inst, err)

	if googleapi.IsNotModified(err) {
		logrus.Warnf("Instance not modified since insert.")
	}

	host := abstract.NewHost()
	host.ID = strconv.FormatUint(inst.Id, 10)
	host.Name = inst.Name

	return host, nil
}

// InspectHost returns the host identified by ref (name or id) or by a *abstract.Host containing an id
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

	hostRef := host.Name
	if hostRef == "" {
		hostRef = host.ID
	}

	if utils.IsEmpty(host) {
		return nil, abstract.ResourceNotFoundError("host", hostRef)
	}

	gcpHost, err := s.ComputeService.Instances.Get(s.GcpConfig.ProjectID, s.GcpConfig.Zone, hostRef).Do()
	if err != nil {
		return nil, err
	}

	host.LastState, err = stateConvert(gcpHost.Status)
	if err != nil {
		return nil, err
	}

	host.Name = gcpHost.Name

	var subnets []IPInSubnet

	for _, nit := range gcpHost.NetworkInterfaces {
		snet := genURL(nit.Subnetwork)
		if !utils.IsEmpty(snet) {
			pubIP := ""
			for _, aco := range nit.AccessConfigs {
				if aco != nil {
					if aco.NatIP != "" {
						pubIP = aco.NatIP
					}
				}
			}

			subnets = append(
				subnets, IPInSubnet{
					Subnet:   snet,
					IP:       nit.NetworkIP,
					PublicIP: pubIP,
				},
			)
		}
	}

	var resouceNetworks []IPInSubnet
	for _, sn := range subnets {
		region, err := getRegionFromSelfLink(sn.Subnet)
		if err != nil {
			continue
		}
		psg, err := s.ComputeService.Subnetworks.Get(
			s.GcpConfig.ProjectID, region, getResourceNameFromSelfLink(sn.Subnet),
		).Do()
		if err != nil {
			continue
		}

		resouceNetworks = append(
			resouceNetworks, IPInSubnet{
				Subnet:   sn.Subnet,
				Name:     psg.Name,
				ID:       strconv.FormatUint(psg.Id, 10),
				IP:       sn.IP,
				PublicIP: sn.PublicIP,
			},
		)
	}

	ip4bynetid := make(map[string]string)
	netnamebyid := make(map[string]string)
	netidbyname := make(map[string]string)

	ipv4 := ""
	for _, rn := range resouceNetworks {
		ip4bynetid[rn.ID] = rn.IP
		netnamebyid[rn.ID] = rn.Name
		netidbyname[rn.Name] = rn.ID
		if rn.PublicIP != "" {
			ipv4 = rn.PublicIP
		}
	}

	err = host.Properties.LockForWrite(hostproperty.NetworkV1).ThenUse(
		func(clonable data.Clonable) error {
			hostNetworkV1 := clonable.(*propsv1.HostNetwork)
			hostNetworkV1.IPv4Addresses = ip4bynetid
			hostNetworkV1.IPv6Addresses = make(map[string]string)
			hostNetworkV1.NetworksByID = netnamebyid
			hostNetworkV1.NetworksByName = netidbyname
			if hostNetworkV1.PublicIPv4 == "" {
				hostNetworkV1.PublicIPv4 = ipv4
			}
			return nil
		},
	)
	if err != nil {
		return nil, fail.Errorf(fmt.Sprintf("failed to update hostproperty.NetworkV1 : %s", err.Error()), err)
	}

	allocated := fromMachineTypeToAllocatedSize(gcpHost.MachineType)

	err = host.Properties.LockForWrite(hostproperty.SizingV1).ThenUse(
		func(clonable data.Clonable) error {
			hostSizingV1 := clonable.(*propsv1.HostSizing)
			hostSizingV1.AllocatedSize.Cores = allocated.Cores
			hostSizingV1.AllocatedSize.RAMSize = allocated.RAMSize
			hostSizingV1.AllocatedSize.DiskSize = allocated.DiskSize
			return nil
		},
	)
	if err != nil {
		return nil, fail.Errorf(fmt.Sprintf("failed to update hostproperty.SizingV1 : %s", err.Error()), err)
	}

	if !host.OK() {
		logrus.Warnf("[TRACE] Unexpected host status: %s", spew.Sdump(host))
	}

	return host, nil
}

func fromMachineTypeToAllocatedSize(machineType string) propsv1.HostSize {
	hz := propsv1.HostSize{}

	// FIXME: Implement mapping

	return hz
}

func stateConvert(gcpHostStatus string) (hoststate.Enum, fail.Error) {
	switch gcpHostStatus {
	case "PROVISIONING":
		return hoststate.STARTING, nil
	case "REPAIRING":
		return hoststate.ERROR, nil
	case "RUNNING":
		return hoststate.STARTED, nil
	case "STAGING":
		return hoststate.STARTING, nil
	case "STOPPED":
		return hoststate.STOPPED, nil
	case "STOPPING":
		return hoststate.STOPPING, nil
	case "SUSPENDED":
		return hoststate.STOPPED, nil
	case "SUSPENDING":
		return hoststate.STOPPING, nil
	case "TERMINATED":
		return hoststate.STOPPED, nil
	default:
		return -1, fail.Errorf(fmt.Sprintf("unexpected host status: [%s]", gcpHostStatus), nil)
	}
}

// GetHostByName returns the host identified by ref (name or id)
func (s *Stack) GetHostByName(name string) (*abstract.Host, fail.Error) {
	hosts, err := s.ListHosts()
	if err != nil {
		return nil, err
	}

	for _, host := range hosts {
		if host.Name == name {
			return host, nil
		}
	}

	return nil, abstract.ResourceNotFoundError("host", name)
}

// DeleteHost deletes the host identified by id
func (s *Stack) DeleteHost(id string) (err error) {
	service := s.ComputeService
	projectID := s.GcpConfig.ProjectID
	zone := s.GcpConfig.Zone
	instanceName := id

	_, err = service.Instances.Get(projectID, zone, instanceName).Do()
	if err != nil {
		return err
	}

	op, err := service.Instances.Delete(projectID, zone, instanceName).Do()
	if err != nil {
		return err
	}

	oco := OpContext{
		Operation:    op,
		ProjectID:    projectID,
		Service:      service,
		DesiredState: "DONE",
	}

	err = waitUntilOperationIsSuccessfulOrTimeout(oco, temporal.GetMinDelay(), temporal.GetHostCleanupTimeout())

	waitErr := retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			_, recErr := service.Instances.Get(projectID, zone, instanceName).Do()
			if gerr, ok := recErr.(*googleapi.Error); ok {
				if gerr.Code == 404 {
					return nil
				}
			}
			return fail.Errorf(
				fmt.Sprintf("error waiting for instance [%s] to disappear: [%v]", instanceName, recErr), recErr,
			)
		}, temporal.GetContextTimeout(),
	)

	if waitErr != nil {
		logrus.Error(fail.Cause(waitErr))
	}

	return err
}

// ResizeHost change the template used by an host
func (s *Stack) ResizeHost(id string, request abstract.SizingRequirements) (*abstract.Host, fail.Error) {
	return nil, fail.NotImplementedError("ResizeHost() not implemented yet") // FIXME: Technical debt
}

// ListHosts lists available hosts
func (s *Stack) ListHosts() ([]*abstract.Host, fail.Error) {
	compuService := s.ComputeService

	var hostList []*abstract.Host

	token := ""
	for paginate := true; paginate; {
		resp, err := compuService.Instances.List(s.GcpConfig.ProjectID, s.GcpConfig.Zone).PageToken(token).Do()
		if err != nil {
			return hostList, fail.Errorf(fmt.Sprintf("cannot list hosts: %v", err), err)
		}
		for _, instance := range resp.Items {
			nhost := abstract.NewHost()
			nhost.ID = strconv.FormatUint(instance.Id, 10)
			nhost.Name = instance.Name
			nhost.LastState, _ = stateConvert(instance.Status)

			hostList = append(hostList, nhost)
		}
		token := resp.NextPageToken
		paginate = token != ""
	}

	return hostList, nil
}

// StopHost stops the host identified by id
func (s *Stack) StopHost(id string) error {
	service := s.ComputeService

	op, err := service.Instances.Stop(s.GcpConfig.ProjectID, s.GcpConfig.Zone, id).Do()
	if err != nil {
		return err
	}

	oco := OpContext{
		Operation:    op,
		ProjectID:    s.GcpConfig.ProjectID,
		Service:      service,
		DesiredState: "DONE",
	}

	err = waitUntilOperationIsSuccessfulOrTimeout(oco, temporal.GetMinDelay(), temporal.GetHostTimeout())
	return err
}

// StartHost starts the host identified by id
func (s *Stack) StartHost(id string) error {
	service := s.ComputeService

	op, err := service.Instances.Start(s.GcpConfig.ProjectID, s.GcpConfig.Zone, id).Do()
	if err != nil {
		return err
	}

	oco := OpContext{
		Operation:    op,
		ProjectID:    s.GcpConfig.ProjectID,
		Service:      service,
		DesiredState: "DONE",
	}

	err = waitUntilOperationIsSuccessfulOrTimeout(oco, temporal.GetMinDelay(), temporal.GetHostTimeout())
	return err
}

// RebootHost reboot the host identified by id
func (s *Stack) RebootHost(id string) error {
	service := s.ComputeService

	op, err := service.Instances.Stop(s.GcpConfig.ProjectID, s.GcpConfig.Zone, id).Do()
	if err != nil {
		return err
	}

	oco := OpContext{
		Operation:    op,
		ProjectID:    s.GcpConfig.ProjectID,
		Service:      service,
		DesiredState: "DONE",
	}

	err = waitUntilOperationIsSuccessfulOrTimeout(oco, temporal.GetMinDelay(), temporal.GetHostTimeout())
	if err != nil {
		return err
	}

	op, err = service.Instances.Start(s.GcpConfig.ProjectID, s.GcpConfig.Zone, id).Do()
	if err != nil {
		return err
	}

	oco = OpContext{
		Operation:    op,
		ProjectID:    s.GcpConfig.ProjectID,
		Service:      service,
		DesiredState: "DONE",
	}

	err = waitUntilOperationIsSuccessfulOrTimeout(oco, temporal.GetMinDelay(), temporal.GetHostTimeout())
	return err
}

// GetHostState returns the host identified by id
func (s *Stack) GetHostState(hostParam interface{}) (hoststate.Enum, fail.Error) {
	host, err := s.InspectHost(hostParam)
	if err != nil {
		return hoststate.ERROR, err
	}

	return host.LastState, nil
}

// -------------Provider Infos-------------------------------------------------------------------------------------------

// ListAvailabilityZones lists the usable AvailabilityZones
func (s *Stack) ListAvailabilityZones() (map[string]bool, fail.Error) {
	zones := make(map[string]bool)

	resp, err := s.ComputeService.Zones.List(s.GcpConfig.ProjectID).Do()
	if err != nil {
		return zones, err
	}
	for _, region := range resp.Items {
		zones[region.Name] = region.Status == "UP"
	}

	return zones, nil
}

// ListRegions ...
func (s *Stack) ListRegions() ([]string, fail.Error) {
	var regions []string

	compuService := s.ComputeService

	resp, err := compuService.Regions.List(s.GcpConfig.ProjectID).Do()
	if err != nil {
		return regions, err
	}
	for _, region := range resp.Items {
		regions = append(regions, region.Name)
	}

	return regions, nil
}
