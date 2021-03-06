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

package abstract

import (
	"github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract/enums/ipversion"
	"github.com/CS-SI/SafeScale/lib/utils/data"
	"github.com/CS-SI/SafeScale/lib/utils/serialize"
)

// GatewayRequest to create a Gateway into a network
type GatewayRequest struct {
	Network *Network
	CIDR    string
	// TemplateID the UUID of the template used to size the host (see SelectTemplates)
	TemplateID string
	// ImageID is the UUID of the image that contains the server's OS and initial state.
	ImageID string
	KeyPair *KeyPair
	// Name is the name to give to the gateway
	Name string

	// OriginalOsRequest is the original os requested
	OriginalOsRequest string
}

// NetworkRequest represents network requirements to create a subnet where Mask is defined in CIDR notation
// like "192.0.2.0/24" or "2001:db8::/32", as defined in RFC 4632 and RFC 4291.
type NetworkRequest struct {
	Name string
	// IPVersion must be IPv4 or IPv6 (see IPVersion)
	IPVersion ipversion.Enum
	// CIDR mask
	CIDR string
	// DNSServers
	DNSServers []string
	// Domain contains the domain used to define host FQDN attached to the network
	Domain string
	// HA tells if 2 gateways and a VIP needs to be created; the VIP IP address will be used as gateway
	HA bool
}

type SubNetwork struct {
	CIDR string `json:"subnetmask,omitempty"`
	ID   string `json:"subnetid,omitempty"`
}

// Network represents a virtual network
type Network struct {
	ID                 string                    `json:"id,omitempty"`                   // ID for the network (from provider)
	Name               string                    `json:"name,omitempty"`                 // Name of the network
	CIDR               string                    `json:"mask,omitempty"`                 // network in CIDR notation
	Domain             string                    `json:"domain,omitempty"`               // contains the domain used to define host FQDN
	GatewayID          string                    `json:"gateway_id,omitempty"`           // contains the id of the host acting as primary gateway for the network
	SecondaryGatewayID string                    `json:"secondary_gateway_id,omitempty"` // contains the id of the host acting as secondary gateway for the network
	VIP                *VirtualIP                `json:"vip,omitempty"`                  // contains the VIP of the network if created with HA
	IPVersion          ipversion.Enum            `json:"ip_version,omitempty"`           // IPVersion is IPv4 or IPv6 (see IPVersion)
	Properties         *serialize.JSONProperties `json:"properties,omitempty"`           // contains optional supplemental information

	Subnetworks []SubNetwork `json:"subnetworks,omitempty"` // FIXME: comment!

	Subnet bool   // FIXME: comment!
	Parent string // FIXME: comment!
}

// NewNetwork ...
func NewNetwork() *Network {
	return &Network{
		Properties: serialize.NewJSONProperties("abstract.network"),
	}
}

// OK ...
func (n *Network) OK() bool {
	result := true
	if n == nil {
		return false
	}

	result = result && (n.ID != "")
	if n.ID == "" {
		logrus.Debug("Network without ID")
	}
	result = result && (n.Name != "")
	if n.Name == "" {
		logrus.Debug("Network without name")
	}
	result = result && (n.CIDR != "")
	if n.CIDR == "" {
		logrus.Debug("Network without CIDR")
	}
	result = result && (n.GatewayID != "")
	if n.GatewayID == "" {
		logrus.Debug("Network without Gateway")
	}
	result = result && (n.Properties != nil)

	return result
}

// Serialize serializes Host instance into bytes (output json code)
func (n *Network) Serialize() ([]byte, error) {
	return serialize.ToJSON(n)
}

// Deserialize reads json code and reinstantiates an Host
func (n *Network) Deserialize(buf []byte) error {
	if n.Properties == nil {
		n.Properties = serialize.NewJSONProperties("abstract.network")
	} else {
		n.Properties.SetModule("abstract.network")
	}
	err := serialize.FromJSON(buf, n)
	if err != nil {
		return err
	}

	return nil
}

// VirtualIP is a structure containing information needed to manage VIP (virtual IP)
type VirtualIP struct {
	ID         string
	Name       string
	NetworkID  string
	PrivateIP  string
	PublicIP   string
	PublicIPID string
	Hosts      []string
}

// NewVirtualIP ...
func NewVirtualIP() *VirtualIP {
	return &VirtualIP{}
}

// Reset ...
func (vip *VirtualIP) Reset() {
	*vip = VirtualIP{}
}

// Content ...
// satisfies interface data.Clonable
func (vip *VirtualIP) Content() data.Clonable {
	return vip
}

// Clone ...
// satisfies interface data.Clonable
func (vip *VirtualIP) Clone() data.Clonable {
	return NewVirtualIP().Replace(vip)
}

// Replace ...
// satisfies interface data.Clonable
func (vip *VirtualIP) Replace(p data.Clonable) data.Clonable {
	if p != nil {
		src := p.(*VirtualIP)
		*vip = *src
		vip.Hosts = make([]string, len(src.Hosts))
		copy(vip.Hosts, src.Hosts)
	}
	return vip
}
