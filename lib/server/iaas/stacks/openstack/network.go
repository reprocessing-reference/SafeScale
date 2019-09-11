/*
 * Copyright 2018-2019, CS Systemes d'Information, http://www.c-s.fr
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

package openstack

import (
	"fmt"
	"net"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"

	"github.com/CS-SI/SafeScale/lib/server/iaas/resources"
	"github.com/CS-SI/SafeScale/lib/server/iaas/resources/enums/HostProperty"
	"github.com/CS-SI/SafeScale/lib/server/iaas/resources/enums/IPVersion"
	propsv1 "github.com/CS-SI/SafeScale/lib/server/iaas/resources/properties/v1"
	"github.com/CS-SI/SafeScale/lib/server/iaas/resources/userdata"
	"github.com/CS-SI/SafeScale/lib/utils"
	"github.com/CS-SI/SafeScale/lib/utils/retry"
)

// RouterRequest represents a router request
type RouterRequest struct {
	Name string `json:"name,omitempty"`
	//NetworkID is the Network ID which the router gateway is connected to.
	NetworkID string `json:"network_id,omitempty"`
}

// Router represents a router
type Router struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	//NetworkID is the Network ID which the router gateway is connected to.
	NetworkID string `json:"network_id,omitempty"`
}

// Subnet define a sub network
type Subnet struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	//IPVersion is IPv4 or IPv6 (see IPVersion)
	IPVersion IPVersion.Enum `json:"ip_version,omitempty"`
	//Mask mask in CIDR notation
	Mask string `json:"mask,omitempty"`
	//NetworkID id of the parent network
	NetworkID string `json:"network_id,omitempty"`
}

// CreateNetwork creates a network named name
func (s *Stack) CreateNetwork(req resources.NetworkRequest) (*resources.Network, error) {
	defer utils.TimerWithLevel(fmt.Sprintf("stacks.openstack::CreateNetwork(%s) called", req.Name), log.TraceLevel)()

	if s == nil {
		panic("Calling stacks.openstack::CreateNetwork from nil pointer!")
	}

	// Checks if CIDR is valid...
	_, _, err := net.ParseCIDR(req.CIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnet '%s (%s)': %s", req.Name, req.CIDR, err.Error())
	}

	// We specify a name and that it should forward packets
	state := true
	opts := networks.CreateOpts{
		Name:         req.Name,
		AdminStateUp: &state,
	}

	// Execute the operation and get back a networks.NetworkClient struct
	network, err := networks.Create(s.NetworkClient, opts).Extract()
	if err != nil {
		return nil, fmt.Errorf("error creating network '%s': %s", req.Name, ProviderErrorToString(err))
	}

	// Starting from here, delete network if exit with error
	defer func() {
		if err != nil {
			derr := networks.Delete(s.NetworkClient, network.ID).ExtractErr()
			if derr != nil {
				log.Errorf("failed to delete network '%s': %v", req.Name, derr)
			}
		}
	}()

	subnet, err := s.createSubnet(req.Name, network.ID, req.CIDR, req.IPVersion, req.DNSServers)
	if err != nil {
		return nil, fmt.Errorf("error creating network '%s': %s", req.Name, ProviderErrorToString(err))
	}

	// Starting from here, delete subnet if exit with error
	defer func() {
		if err != nil {
			derr := s.deleteSubnet(subnet.ID)
			if derr != nil {
				log.Errorf("failed to delete subnet '%s': %+v", subnet.ID, derr)
			}
		}
	}()

	newNet := resources.NewNetwork()
	newNet.ID = network.ID
	newNet.Name = network.Name
	newNet.CIDR = subnet.Mask
	newNet.IPVersion = subnet.IPVersion
	return newNet, nil
}

// GetNetworkByName ...
func (s *Stack) GetNetworkByName(name string) (*resources.Network, error) {
	defer utils.TimerWithLevel(fmt.Sprintf("stacks.openstack::GetNetworkByName(%s) called", name), log.TraceLevel)()

	if s == nil {
		panic("Calling stacks.openstack::GetNetworkByName from nil pointer!")
	}
	if name == "" {
		panic("name is empty!")
	}

	// Gophercloud doesn't propose the way to get a host by name, but OpenStack knows how to do it...
	r := networks.GetResult{}
	_, r.Err = s.ComputeClient.Get(s.NetworkClient.ServiceURL("networks?name="+name), &r.Body, &gophercloud.RequestOpts{
		OkCodes: []int{200, 203},
	})
	if r.Err != nil {
		if _, ok := r.Err.(gophercloud.ErrDefault403); ok {
			return nil, resources.ResourceAccessDeniedError("network", name)
		}
		return nil, fmt.Errorf("query for network '%s' failed: %v", name, r.Err)
	}
	nets, found := r.Body.(map[string]interface{})["networks"].([]interface{})
	if found && len(nets) > 0 {
		entry := nets[0].(map[string]interface{})
		id := entry["id"].(string)
		return s.GetNetwork(id)
	}
	return nil, resources.ResourceNotFoundError("network", name)
}

// GetNetwork returns the network identified by id
func (s *Stack) GetNetwork(id string) (*resources.Network, error) {
	defer utils.TimerWithLevel(fmt.Sprintf("stacks.openstack::GetNetwork(%s) called", id), log.TraceLevel)()

	if s == nil {
		panic("Calling stacks.openstack::GetNetwork from nil pointer!")
	}

	// If not found, we look for any network from provider
	// 1st try with id
	network, err := networks.Get(s.NetworkClient, id).Extract()
	if err != nil {
		if _, ok := err.(gophercloud.ErrDefault404); !ok {
			// log.Errorf("Error getting network: %+v", err)
			return nil, errors.Wrap(err, fmt.Sprintf("Error getting network '%s': %s", id, ProviderErrorToString(err)))
		}
	}
	if network != nil && network.ID != "" {
		sns, err := s.listSubnets(id)
		if err != nil {
			// log.Errorf("Error getting network: listing subnet: %+v", err)
			return nil, errors.Wrap(err, fmt.Sprintf("Error getting network: %s", ProviderErrorToString(err)))
		}
		if len(sns) != 1 {
			return nil, fmt.Errorf("Bad configuration, each network should have exactly one subnet")
		}
		sn := sns[0]
		// gwID, _ := client.getGateway(id)
		// if err != nil {
		// 	return nil, fmt.Errorf("Bad configuration, no gateway associated to this network")
		// }
		newNet := resources.NewNetwork()
		newNet.ID = network.ID
		newNet.Name = network.Name
		newNet.CIDR = sn.Mask
		newNet.IPVersion = sn.IPVersion
		//net.GatewayID = network.GatewayId
		return newNet, nil
	}

	// At this point, no network has been found with given reference
	log.Debugf(resources.ResourceNotFoundError("network(GetNetwork)", id).Error())
	return nil, errors.Wrap(resources.ResourceNotFoundError("network(GetNetwork)", id), "")
}

// ListNetworks lists available networks
func (s *Stack) ListNetworks() ([]*resources.Network, error) {
	defer utils.TimerWithLevel(fmt.Sprintf("stacks.openstack::ListNetworks() called"), log.TraceLevel)()

	if s == nil {
		panic("Calling stacks.openstack::ListNetworks from nil pointer!")
	}

	// Retrieve a pager (i.e. a paginated collection)
	var netList []*resources.Network
	pager := networks.List(s.NetworkClient, networks.ListOpts{})
	err := pager.EachPage(
		func(page pagination.Page) (bool, error) {
			networkList, err := networks.ExtractNetworks(page)
			if err != nil {
				return false, err
			}

			for _, n := range networkList {
				sns, err := s.listSubnets(n.ID)
				if err != nil {
					return false, fmt.Errorf("Error getting network: %s", ProviderErrorToString(err))
				}
				if len(sns) != 1 {
					continue
				}
				if n.ID == s.ProviderNetworkID {
					continue
				}
				sn := sns[0]

				newNet := resources.NewNetwork()
				newNet.ID = n.ID
				newNet.Name = n.Name
				newNet.CIDR = sn.Mask
				newNet.IPVersion = sn.IPVersion
				// GatewayID: gwID,
				netList = append(netList, newNet)
			}
			return true, nil
		},
	)
	if len(netList) == 0 || err != nil {
		if err != nil {
			log.Debugf("Error listing networks: pagination error: %+v", err)
			return nil, errors.Wrap(err, fmt.Sprintf("Error listing networks: %s", ProviderErrorToString(err)))
		}
		log.Debugf("Listing all networks: Empty network list !")
	}
	return netList, nil
}

// DeleteNetwork deletes the network identified by id
func (s *Stack) DeleteNetwork(id string) error {
	defer utils.TimerWithLevel(fmt.Sprintf("stacks.openstack::DeleteNetwork(%s) called", id), log.TraceLevel)()

	if s == nil {
		panic("Calling stacks.openstack::DeleteNetwork from nil pointer!")
	}

	network, err := networks.Get(s.NetworkClient, id).Extract()
	if err != nil {
		log.Errorf("Failed to delete network: %+v", err)
		if strings.Contains(err.Error(), "Resource not found") {
			log.Errorf("Inconsistent network data !!")
		}
	}

	sns, err := s.listSubnets(id)
	if err != nil {
		msg := fmt.Sprintf("failed to delete network '%s': %s", network.Name, ProviderErrorToString(err))
		log.Debugf(utils.Capitalize(msg))
		return fmt.Errorf(msg)
	}
	for _, sn := range sns {
		err := s.deleteSubnet(sn.ID)
		if err != nil {
			switch err.(type) {
			case resources.ErrResourceNotAvailable:
				return err
			default:
				msg := fmt.Sprintf("failed to delete network '%s': %s", network.Name, ProviderErrorToString(err))
				log.Debugf(utils.Capitalize(msg))
				return fmt.Errorf(msg)
			}
		}
	}
	err = networks.Delete(s.NetworkClient, id).ExtractErr()
	if err != nil {
		switch err.(type) {
		case resources.ErrResourceNotAvailable:
			return err
		default:
			msg := fmt.Sprintf("failed to delete network '%s': %s", network.Name, ProviderErrorToString(err))
			log.Debugf(utils.Capitalize(msg))
			return fmt.Errorf(msg)
		}
	}

	return nil
}

// CreateGateway creates a public Gateway for a private network
func (s *Stack) CreateGateway(req resources.GatewayRequest) (*resources.Host, *userdata.Content, error) {
	defer utils.TimerWithLevel(fmt.Sprintf("stacks.openstack::CreateGateway(%s) called...", req.Name), log.TraceLevel)()
	if s == nil {
		panic("Calling stacks.openstack::CreateGateway from nil pointer!")
	}

	userData := userdata.NewContent()

	// Ensure network exists
	if req.Network == nil {
		panic("req.Network is nil!")
	}
	gwname := req.Name
	if gwname == "" {
		gwname = "gw-" + req.Network.Name
	}

	password, err := utils.GeneratePassword(16)
	if err != nil {
		return nil, userData, fmt.Errorf("failed to generate password: %s", err.Error())
	}
	hostReq := resources.HostRequest{
		ImageID:      req.ImageID,
		KeyPair:      req.KeyPair,
		ResourceName: gwname,
		TemplateID:   req.TemplateID,
		Networks:     []*resources.Network{req.Network},
		PublicIP:     true,
		Password:     password,
	}
	host, userData, err := s.CreateHost(hostReq)
	if err != nil {
		log.Errorf("Error creating gateway: creating host: %+v", err)
		return nil, userData, errors.Wrap(err, fmt.Sprintf("Error creating gateway : %s", ProviderErrorToString(err)))
	}

	// delete the host when found problem starting from here
	defer func() {
		if err != nil {
			derr := s.DeleteHost(host.ID)
			if derr != nil {
				log.Errorf("failed to delete host '%s': %v", host.Name, derr)
			}
		}
	}()

	// Updates Host Property propsv1.HostSizing
	err = host.Properties.LockForWrite(HostProperty.SizingV1).ThenUse(func(v interface{}) error {
		hostSizingV1 := v.(*propsv1.HostSizing)
		hostSizingV1.Template = req.TemplateID
		return nil
	})
	if err != nil {
		return nil, userData, errors.Wrap(err, fmt.Sprintf("Error creating gateway : %s", ProviderErrorToString(err)))
	}
	return host, userData, nil
}

// DeleteGateway delete the public gateway of a private network
func (s *Stack) DeleteGateway(id string) error {
	defer utils.TimerWithLevel(fmt.Sprintf("stacks.openstack::DeleteGateway(%s) called", id), log.TraceLevel)()

	if s == nil {
		panic("Calling stacks.openstack::DeleteGateway from nil pointer!")
	}

	return s.DeleteHost(id)
}

// ToGopherIPversion ...
func ToGopherIPversion(v IPVersion.Enum) gophercloud.IPVersion {
	if v == IPVersion.IPv4 {
		return gophercloud.IPv4
	} else if v == IPVersion.IPv6 {
		return gophercloud.IPv6
	}
	return -1
}

func fromGopherIPversion(v gophercloud.IPVersion) IPVersion.Enum {
	if v == gophercloud.IPv4 {
		return IPVersion.IPv4
	} else if v == gophercloud.IPv6 {
		return IPVersion.IPv6
	}
	return -1
}

// FromIntIPversion ...
func FromIntIPversion(v int) IPVersion.Enum {
	if v == 4 {
		return IPVersion.IPv4
	} else if v == 6 {
		return IPVersion.IPv6
	}
	return -1
}

// createSubnet creates a sub network
// - netID ID of the parent network
// - name is the name of the sub network
// - mask is a network mask defined in CIDR notation
func (s *Stack) createSubnet(name string, networkID string, cidr string, ipVersion IPVersion.Enum, dnsServers []string) (*Subnet, error) {
	// You must associate a new subnet with an existing network - to do this you
	// need its UUID. You must also provide a well-formed CIDR value.
	dhcp := true
	opts := subnets.CreateOpts{
		NetworkID:  networkID,
		CIDR:       cidr,
		IPVersion:  ToGopherIPversion(ipVersion),
		Name:       name,
		EnableDHCP: &dhcp,
	}

	// // Try to reserve IP addresses as static reserve at the beginning of the CIDR, and adapt DHCP allocation pool
	// dhcpPoolStart, dhcpPoolEnd, err := calcDhcpAllocationPool(cidr)
	// var ok bool
	// if err != nil {
	// 	if _, ok = err.(utils.ErrOverflow); !ok {
	// 		return nil, err
	// 	}
	// }
	// if err == nil || ok {
	// 	dhcpAllocationPool := subnets.AllocationPool{
	// 		Start: dhcpPoolStart,
	// 		End:   dhcpPoolEnd,
	// 	}
	// 	opts.AllocationPools = []subnets.AllocationPool{dhcpAllocationPool}
	// }

	if len(dnsServers) > 0 {
		opts.DNSNameservers = dnsServers
	}

	if !s.cfgOpts.UseLayer3Networking {
		noGateway := ""
		opts.GatewayIP = &noGateway
	}

	// Execute the operation and get back a subnets.Subnet struct
	r := subnets.Create(s.NetworkClient, opts)
	subnet, err := r.Extract()
	if err != nil {
		switch r.Err.(type) {
		case gophercloud.ErrDefault400:
			neutronError := ParseNeutronError(r.Err.Error())
			if neutronError != nil {
				msg := fmt.Sprintf("Error creating subnet: bad request: %s\n", neutronError["message"])
				log.Debugf(msg)
				return nil, fmt.Errorf(msg)
			}
		}
		log.Debugf("Error creating subnet: %+v\n", err)
		return nil, errors.Wrap(err, fmt.Sprintf("Error creating subnet: %s", ProviderErrorToString(err)))
	}

	// Starting from here, delete subnet if exit with error
	defer func() {
		if err != nil {
			derr := s.deleteSubnet(subnet.ID)
			if derr != nil {
				log.Warnf("Error deleting subnet: %v", derr)
			}
		}
	}()

	if s.cfgOpts.UseLayer3Networking {
		router, err := s.createRouter(RouterRequest{
			Name:      subnet.ID,
			NetworkID: s.ProviderNetworkID,
		})
		if err != nil {
			log.Debugf("Error creating subnet: %+v", err)
			return nil, errors.Wrap(err, fmt.Sprintf("Error creating subnet: %s", ProviderErrorToString(err)))
		}

		// Starting from here, delete router if exit with error
		defer func() {
			if err != nil {
				derr := s.deleteRouter(router.ID)
				if derr != nil {
					log.Warnf("Error deleting router: %v", derr)
				}
			}
		}()

		err = s.addSubnetToRouter(router.ID, subnet.ID)
		if err != nil {
			log.Debugf("Error creating subnet: %+v", err)
			return nil, errors.Wrap(err, fmt.Sprintf("Error creating subnet: %s", ProviderErrorToString(err)))
		}
	}

	return &Subnet{
		ID:        subnet.ID,
		Name:      subnet.Name,
		IPVersion: FromIntIPversion(subnet.IPVersion),
		Mask:      subnet.CIDR,
		NetworkID: subnet.NetworkID,
	}, nil
}

// calcDhcpAllocationPool calculates the start and the end of the DHCP allocation pool based on cidr
// Keeps 10 IP addresses out of the pool to allow static and VIP addresses
func calcDhcpAllocationPool(cidr string) (string, string, error) {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", fmt.Errorf("Invalid cidr '%s'", cidr)
	}

	start, end, err := utils.CIDRToLongRange(cidr)
	if err != nil {
		return "", "", err
	}
	start += 11
	if start >= end {
		return "", "", utils.OverflowError(fmt.Sprintf("Not enough IP Addresses in CIDR '%s' to reserve 10 static IP addresses", cidr))
	}
	end -= 3
	if end <= start {
		return "", "", utils.OverflowError(fmt.Sprintf("Not enough IP Addresses in CIDR '%s' to reserve 10 static IP addresses", cidr))
	}
	return utils.LongToIPv4(start), utils.LongToIPv4(end), nil
}

// getSubnet returns the sub network identified by id
func (s *Stack) getSubnet(id string) (*Subnet, error) {
	// Execute the operation and get back a subnets.Subnet struct
	subnet, err := subnets.Get(s.NetworkClient, id).Extract()
	if err != nil {
		log.Debugf("Error getting subnet: getting subnet: %+v", err)
		return nil, errors.Wrap(err, fmt.Sprintf("Error getting subnet: %s", ProviderErrorToString(err)))
	}
	return &Subnet{
		ID:        subnet.ID,
		Name:      subnet.Name,
		IPVersion: FromIntIPversion(subnet.IPVersion),
		Mask:      subnet.CIDR,
		NetworkID: subnet.NetworkID,
	}, nil
}

// listSubnets lists available sub networks of network net
func (s *Stack) listSubnets(netID string) ([]Subnet, error) {
	pager := subnets.List(s.NetworkClient, subnets.ListOpts{
		NetworkID: netID,
	})
	var subnetList []Subnet
	paginationErr := pager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := subnets.ExtractSubnets(page)
		if err != nil {
			return false, fmt.Errorf("Error listing subnets: %s", ProviderErrorToString(err))
		}

		for _, subnet := range list {
			subnetList = append(subnetList, Subnet{
				ID:        subnet.ID,
				Name:      subnet.Name,
				IPVersion: FromIntIPversion(subnet.IPVersion),
				Mask:      subnet.CIDR,
				NetworkID: subnet.NetworkID,
			})
		}
		return true, nil
	})

	if (paginationErr != nil) || (len(subnetList) == 0) {
		if paginationErr != nil {
			log.Debugf("Error listing subnets: pagination error: %+v", paginationErr)
			return nil, errors.Wrap(paginationErr, fmt.Sprintf("we have a pagination error !: %v", paginationErr))
		}
	}

	return subnetList, nil
}

// deleteSubnet deletes the sub network identified by id
func (s *Stack) deleteSubnet(id string) error {
	routerList, _ := s.ListRouters()
	var router *Router
	for _, r := range routerList {
		if r.Name == id {
			router = &r
			break
		}
	}
	if router != nil {
		if err := s.removeSubnetFromRouter(router.ID, id); err != nil {
			msg := fmt.Sprintf("failed to delete subnet '%s': %s", id, ProviderErrorToString(err))
			log.Debug(utils.Capitalize(msg))
			return fmt.Errorf(msg)
		}
		if err := s.deleteRouter(router.ID); err != nil {
			msg := fmt.Sprintf("failed to delete subnet '%s': %s", id, ProviderErrorToString(err))
			log.Debug(utils.Capitalize(msg))
			return fmt.Errorf(msg)
		}
	}

	var err error
	retryErr := retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			r := subnets.Delete(s.NetworkClient, id)
			err = r.ExtractErr()
			if err != nil {
				if _, ok := err.(gophercloud.ErrUnexpectedResponseCode); ok {
					neutronError := ParseNeutronError(err.Error())
					switch neutronError["type"] {
					case "SubnetInUse":
						msg := fmt.Sprintf("hosts or services are still attached")
						log.Warnf(utils.Capitalize(msg))
						return resources.ResourceNotAvailableError("subnet", id)
					default:
						log.Debugf("NeutronError: type = %s", neutronError["type"])
					}
				} else {
					msg := fmt.Sprintf("failed to delete subnet '%s': %s", id, ProviderErrorToString(err))
					log.Errorf(utils.Capitalize(msg))
					return fmt.Errorf(msg)
				}
			}
			return nil
		},
		utils.GetContextTimeout(),
	)
	if retryErr != nil {
		if _, ok := retryErr.(retry.ErrTimeout); ok {
			// If we have the last error of the delete try, returns this error
			if err != nil {
				if _, ok := err.(resources.ErrResourceNotAvailable); ok {
					return err
				}
				return fmt.Errorf("failed to delete subnet after %v: %v", utils.GetContextTimeout(), err)
			}
		}
		return fmt.Errorf("failed to delete subnet after %v: %v", utils.GetContextTimeout(), retryErr)
	}
	return nil
}

// createRouter creates a router satisfying req
func (s *Stack) createRouter(req RouterRequest) (*Router, error) {
	// Create a router to connect external Provider network
	gi := routers.GatewayInfo{
		NetworkID: req.NetworkID,
	}
	state := true
	opts := routers.CreateOpts{
		Name:         req.Name,
		AdminStateUp: &state,
		GatewayInfo:  &gi,
	}
	router, err := routers.Create(s.NetworkClient, opts).Extract()
	if err != nil {
		log.Debugf("failed to create router '%s': %+v", req.Name, err)
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create router '%s': %s", req.Name, ProviderErrorToString(err)))
	}
	log.Debugf("Router '%s' (%s) successfully created", router.Name, router.ID)
	return &Router{
		ID:        router.ID,
		Name:      router.Name,
		NetworkID: router.GatewayInfo.NetworkID,
	}, nil
}

// getRouter returns the router identified by id
func (s *Stack) getRouter(id string) (*Router, error) {
	r, err := routers.Get(s.NetworkClient, id).Extract()
	if err != nil {
		log.Debugf("Error getting router '%s': %+v", id, err)
		return nil, errors.Wrap(err, fmt.Sprintf("Error getting Router: %s", ProviderErrorToString(err)))
	}
	return &Router{
		ID:        r.ID,
		Name:      r.Name,
		NetworkID: r.GatewayInfo.NetworkID,
	}, nil
}

// ListRouters lists available routers
func (s *Stack) ListRouters() ([]Router, error) {
	if s == nil {
		panic("Calling openstack.Stack::ListRouters from nil pointer!")
	}

	var ns []Router
	err := routers.List(s.NetworkClient, routers.ListOpts{}).EachPage(
		func(page pagination.Page) (bool, error) {
			list, err := routers.ExtractRouters(page)
			if err != nil {
				return false, err
			}
			for _, r := range list {
				an := Router{
					ID:        r.ID,
					Name:      r.Name,
					NetworkID: r.GatewayInfo.NetworkID,
				}
				ns = append(ns, an)
			}
			return true, nil
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Error listing volume types: %s", ProviderErrorToString(err)))
	}
	return ns, nil
}

// deleteRouter deletes the router identified by id
func (s *Stack) deleteRouter(id string) error {
	err := routers.Delete(s.NetworkClient, id).ExtractErr()
	if err != nil {
		log.Debugf("Error deleting router: %+v", err)
		return errors.Wrap(err, fmt.Sprintf("error deleting router: %s", ProviderErrorToString(err)))
	}
	return nil
}

// addSubnetToRouter attaches subnet to router
func (s *Stack) addSubnetToRouter(routerID string, subnetID string) error {
	_, err := routers.AddInterface(s.NetworkClient, routerID, routers.AddInterfaceOpts{
		SubnetID: subnetID,
	}).Extract()
	if err != nil {
		log.Debugf("Error adding subnet to router: adding interface: %+v", err)
		return errors.Wrap(err, fmt.Sprintf("Error addinter subnet: %s", ProviderErrorToString(err)))
	}
	return nil
}

// removeSubnetFromRouter detaches a subnet from router interface
func (s *Stack) removeSubnetFromRouter(routerID string, subnetID string) error {
	r := routers.RemoveInterface(s.NetworkClient, routerID, routers.RemoveInterfaceOpts{
		SubnetID: subnetID,
	})
	_, err := r.Extract()
	if err != nil {
		msg := fmt.Sprintf("failed to remove subnet '%s' from router '%s': %s", subnetID, routerID, ProviderErrorToString(err))
		log.Debug(msg)
		return errors.Wrap(err, msg)
	}
	return nil
}

// listPorts lists all ports available
func (s *Stack) listPorts(options ports.ListOpts) ([]ports.Port, error) {
	allPages, err := ports.List(s.NetworkClient, options).AllPages()
	if err != nil {
		return nil, err
	}
	return ports.ExtractPorts(allPages)
}

// CreateVIP creates a private virtual IP
// If public is set to true,
func (s *Stack) CreateVIP(networkID string, name string) (*resources.VIP, error) {
	asu := true
	sg := []string{s.SecurityGroup.ID}
	options := ports.CreateOpts{
		NetworkID:      networkID,
		AdminStateUp:   &asu,
		Name:           name,
		SecurityGroups: &sg,
	}
	port, err := ports.Create(s.NetworkClient, options).Extract()
	if err != nil {
		return nil, err
	}
	vip := resources.VIP{
		ID:        port.ID,
		PrivateIP: port.FixedIPs[0].IPAddress,
	}
	return &vip, nil
}

// AddPublicIPToVIP adds a public IP to VIP
func (s *Stack) AddPublicIPToVIP(vip *resources.VIP) error {
	return utils.NotImplementedError("AddPublicIPToVIP() not implemented yet")
}

// BindHostToVIP makes the host passed as parameter an allowed "target" of the VIP
func (s *Stack) BindHostToVIP(vip *resources.VIP, host *resources.Host) error {
	vipPort, err := ports.Get(s.NetworkClient, vip.ID).Extract()
	if err != nil {
		return err
	}
	hostPorts, err := s.listPorts(ports.ListOpts{
		DeviceID:  host.ID,
		NetworkID: vip.NetworkID,
	})
	if err != nil {
		return err
	}
	addressPair := ports.AddressPair{
		MACAddress: vipPort.MACAddress,
		IPAddress:  vip.PrivateIP,
	}
	for _, p := range hostPorts {
		p.AllowedAddressPairs = append(p.AllowedAddressPairs, addressPair)
		_, err = ports.Update(s.NetworkClient, p.ID, ports.UpdateOpts{AllowedAddressPairs: &p.AllowedAddressPairs}).Extract()
		if err != nil {
			return err
		}
	}
	return nil
}

// UnbindHostFromVIP removes the bind between the VIP and a host
func (s *Stack) UnbindHostFromVIP(vip *resources.VIP, host *resources.Host) error {
	vipPort, err := ports.Get(s.NetworkClient, vip.ID).Extract()
	if err != nil {
		return err
	}
	hostPorts, err := s.listPorts(ports.ListOpts{
		DeviceID:  host.ID,
		NetworkID: vip.NetworkID,
	})
	if err != nil {
		return err
	}
	for _, p := range hostPorts {
		newAllowedAddressPairs := []ports.AddressPair{}
		for _, a := range p.AllowedAddressPairs {
			if a.MACAddress != vipPort.MACAddress {
				newAllowedAddressPairs = append(newAllowedAddressPairs, a)
			}
		}
		_, err = ports.Update(s.NetworkClient, p.ID, ports.UpdateOpts{AllowedAddressPairs: &newAllowedAddressPairs}).Extract()
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteVIP deletes the port corresponding to the VIP
func (s *Stack) DeleteVIP(vip *resources.VIP) error {
	for _, h := range vip.Hosts {
		err := s.UnbindHostFromVIP(vip, h)
		if err != nil {
			return err
		}
	}
	return ports.Delete(s.NetworkClient, vip.ID).ExtractErr()
}
