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

package propertiesv2

import (
	"github.com/CS-SI/SafeScale/lib/server/cluster/enums/property"
	"github.com/CS-SI/SafeScale/lib/utils/data"
	"github.com/CS-SI/SafeScale/lib/utils/serialize"
)

// Network replace propertiesv1.Network
// FIXME: make sure there is code to migrate propertiesv1.Network to propertiesv2.Network when needed
// !!! FROZEN !!!
// Note: if tagged as FROZEN, must not be changed ever.
//       Create a new version instead with updated/additional fields
type Network struct {
	NetworkID          string `json:"network_id"`           // contains the ID of the network
	CIDR               string `json:"cidr"`                 // the network CIDR
	GatewayID          string `json:"gateway_id"`           // contains the ID of the primary gateway
	GatewayIP          string `json:"gateway_ip"`           // contains the private IP address of the primary gateway
	SecondaryGatewayID string `json:"secondary_gateway_id"` // contains the ID of the secondary gateway
	SecondaryGatewayIP string `json:"secondary_gateway_ip"` // contains the private IP of the secondary gateway
	DefaultRouteIP     string `json:"default_route_ip"`     // contains the IP of the default route
	PrimaryPublicIP    string `json:"primary_public_ip"`    // contains the public IP of the primary gateway
	SecondaryPublicIP  string `json:"secondary_public_ip"`  // contains the public IP of the secondary gateway
	EndpointIP         string `json:"endpoint_ip"`          // contains the IP of the external Endpoint
	Domain             string `json:"domain,omitempty"`     // contains the domain used to define the host FQDN at creation (taken from the network)
}

func newNetwork() *Network {
	return &Network{}
}

// Content ...
// satisfies interface data.Clonable
func (n *Network) Content() data.Clonable {
	return n
}

// Clone ...
// satisfies interface data.Clonable
func (n *Network) Clone() data.Clonable {
	return newNetwork().Replace(n)
}

// Replace ...
// satisfies interface data.Clonable
func (n *Network) Replace(p data.Clonable) data.Clonable {
	*n = *p.(*Network)
	return n
}

func init() {
	serialize.PropertyTypeRegistry.Register("clusters", property.NetworkV2, &Network{})
}
