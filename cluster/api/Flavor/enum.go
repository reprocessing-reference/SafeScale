/*
 * Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
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

package Flavor

import (
	"fmt"
	"strings"
)

//Enum represents the flavor of a cluster, in other words what technology is used behind the scene
type Enum int

const (
	_ Enum = iota
	// DCOS managed cluster
	DCOS
	// Kubernetes flavor for a pure Kubernetes cluster
	Kubernetes
	// Swarm flavor for a pure docker swarm cluster
	Swarm
	// BOH for a "Bunch Of Hosts", without cluster management
	BOH
)

var (
	stringMap = map[string]Enum{
		"dcos":       DCOS,
		"kubernetes": Kubernetes,
		"swarm":      Swarm,
		"boh":        BOH,
	}

	enumMap = map[Enum]string{
		DCOS:       "DCOS",
		Kubernetes: "Kubernetes",
		Swarm:      "Swarm",
		BOH:        "BOH",
	}
)

// Parse returns a Enum corresponding to the string parameter
// If the string doesn't correspond to any Enum, returns an error (nil otherwise)
// This function is intended to be used to parse user input.
func Parse(v string) (Enum, error) {
	var (
		e  Enum
		ok bool
	)
	lowered := strings.ToLower(v)
	if e, ok = stringMap[lowered]; !ok {
		return e, fmt.Errorf("failed to find a Flavor.Enum corresponding to '%s'", v)
	}
	return e, nil

}

// FromString returns a Enum corresponding to the string parameter
// This method is intended to be used from validated input.
func FromString(v string) (e Enum) {
	e, err := Parse(v)
	if err != nil {
		panic(err.Error())
	}
	return
}

// String returns a string representaton of an Enum
func (e Enum) String() string {
	if str, found := enumMap[e]; found {
		return str
	}
	panic(fmt.Sprintf("failed to find a Flavor.Enum string corresponding to value '%d'!", e))
}
