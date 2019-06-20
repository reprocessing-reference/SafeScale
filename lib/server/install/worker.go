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

package install

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/CS-SI/SafeScale/lib/server/iaas/resources"

	log "github.com/sirupsen/logrus"

	pb "github.com/CS-SI/SafeScale/lib"
	"github.com/CS-SI/SafeScale/lib/client"
	clusterapi "github.com/CS-SI/SafeScale/lib/server/cluster/api"
	"github.com/CS-SI/SafeScale/lib/server/cluster/enums/Complexity"
	"github.com/CS-SI/SafeScale/lib/server/cluster/enums/Flavor"
	"github.com/CS-SI/SafeScale/lib/server/install/enums/Action"
	"github.com/CS-SI/SafeScale/lib/server/install/enums/Method"
	srvutils "github.com/CS-SI/SafeScale/lib/server/utils"
)

const (
	yamlPaceKeyword    = "pace"
	yamlStepsKeyword   = "steps"
	yamlTargetsKeyword = "targets"
	yamlRunKeyword     = "run"
	yamlPackageKeyword = "package"
	yamlOptionsKeyword = "options"
	yamlTimeoutKeyword = "timeout"
	yamlSerialKeyword  = "serialized"
)

type alterCommandCB func(string) string

type worker struct {
	feature   *Feature
	target    Target
	method    Method.Enum
	action    Action.Enum
	variables Variables
	settings  Settings

	host    *pb.Host
	node    bool
	cluster clusterapi.Cluster

	availableMaster  *pb.Host
	availableNode    *pb.Host
	availableGateway *pb.Host

	allMasters  []*pb.Host
	allNodes    []*pb.Host
	allGateways []*pb.Host

	concernedMasters  []*pb.Host
	concernedNodes    []*pb.Host
	concernedGateways []*pb.Host

	rootKey string
	// function to alter the content of 'run' key of specification file
	commandCB alterCommandCB
}

// newWorker ...
// alterCmdCB is used to change the content of keys 'run' or 'package' before executing
// the requested action. If not used, must be nil
func newWorker(f *Feature, t Target, m Method.Enum, a Action.Enum, cb alterCommandCB) (*worker, error) {
	w := worker{
		feature:   f,
		target:    t,
		method:    m,
		action:    a,
		commandCB: cb,
	}
	hT, cT, nT := determineContext(t)
	if cT != nil {
		w.cluster = cT.cluster
	}
	if hT != nil {
		w.host = hT.host
	}
	if nT != nil {
		w.host = nT.host
		w.node = true
	}

	w.rootKey = "feature.install." + strings.ToLower(m.String()) + "." + strings.ToLower(a.String())
	if !f.specs.IsSet(w.rootKey) {
		msg := `syntax error in feature '%s' specification file (%s):
				no key '%s' found`
		return nil, fmt.Errorf(msg, f.DisplayName(), f.DisplayFilename(), w.rootKey)
	}

	return &w, nil
}

// ConcernCluster returns true if the target of the worker is a cluster
func (w *worker) ConcernCluster() bool {
	return w.cluster != nil
}

// CanProceed tells if the combination Feature/Target can work
func (w *worker) CanProceed(s Settings) error {
	if w.cluster != nil {
		err := w.validateContextForCluster()
		if err == nil && !s.SkipSizingRequirements {
			err = w.validateClusterSizing()
		}
		return err
	}
	return w.validateContextForHost()
}

// identifyAvailableGateway finds a gateway available, and keep track of it
// for all the life of the action (prevent to request too often)
// For now, only one gateway is allowed, but in the future we may have 2 for High Availability
func (w *worker) identifyAvailableGateway() (*pb.Host, error) {
	if w.cluster == nil {
		return gatewayFromHost(w.host), nil
	}
	if w.availableGateway == nil {
		var err error
		netCfg := w.cluster.GetNetworkConfig(w.feature.task)
		w.availableGateway, err = client.New().Host.Inspect(netCfg.GatewayID, client.DefaultExecutionTimeout)
		if err != nil {
			return nil, err
		}
	}
	return w.availableGateway, nil
}

// identifyAvailableMaster finds a master available, and keep track of it
// for all the life of the action (prevent to request too often)
func (w *worker) identifyAvailableMaster() (*pb.Host, error) {
	if w.cluster == nil {
		return nil, resources.ResourceNotAvailableError("cluster", "")
	}
	if w.availableMaster == nil {
		hostID, err := w.cluster.FindAvailableMaster(w.feature.task)
		if err != nil {
			return nil, err
		}
		w.availableMaster, err = client.New().Host.Inspect(hostID, client.DefaultExecutionTimeout)
		if err != nil {
			return nil, err
		}
	}
	return w.availableMaster, nil
}

// identifyAvailableNode finds a node available and will use this one during all the install session
func (w *worker) identifyAvailableNode() (*pb.Host, error) {
	if w.cluster == nil {
		return nil, resources.ResourceNotAvailableError("cluster", "")
	}
	if w.availableNode == nil {
		hostID, err := w.cluster.FindAvailableNode(w.feature.task)
		if err != nil {
			return nil, err
		}
		host, err := client.New().Host.Inspect(hostID, client.DefaultExecutionTimeout)
		if err != nil {
			return nil, err
		}
		w.availableNode = host
	}
	return w.availableNode, nil
}

// identifyConcernedMasters returns a list of all the hosts acting as masters and keep this list
// during all the install session
func (w *worker) identifyConcernedMasters() ([]*pb.Host, error) {
	if w.cluster == nil {
		return []*pb.Host{}, nil
	}
	if w.concernedMasters == nil {
		hosts, err := w.identifyAllMasters()
		if err != nil {
			return nil, err
		}
		concernedHosts, err := w.extractHostsFailingCheck(hosts)
		if err != nil {
			return nil, err
		}
		w.concernedMasters = concernedHosts
	}
	return w.concernedMasters, nil
}

// extractHostsFailingCheck identifies from the list passed as parameter which
// hosts fail feature check.
// The checks are done in parallel.
func (w *worker) extractHostsFailingCheck(hosts []*pb.Host) ([]*pb.Host, error) {
	concernedHosts := []*pb.Host{}
	dones := map[*pb.Host]chan error{}
	results := map[*pb.Host]chan Results{}
	for _, h := range hosts {
		d := make(chan error)
		r := make(chan Results)
		dones[h] = d
		results[h] = r
		go func(host *pb.Host, res chan Results, done chan error) {
			nodeTarget := NewNodeTarget(host)
			results, err := w.feature.Check(nodeTarget, w.variables, w.settings)
			if err != nil {
				res <- nil
				done <- err
				return
			}
			res <- results
			done <- nil
		}(h, r, d)
	}
	for h := range dones {
		r := <-results[h]
		d := <-dones[h]
		if d != nil {
			return nil, d
		}
		if !r.Successful() {
			concernedHosts = append(concernedHosts, h)
		}
	}
	return concernedHosts, nil
}

// identifyAllMasters returns a list of all the hosts acting as masters and keep this list
// during all the install session
func (w *worker) identifyAllMasters() ([]*pb.Host, error) {
	if w.cluster == nil {
		return []*pb.Host{}, nil
	}
	if w.allMasters == nil || len(w.allMasters) == 0 {
		w.allMasters = []*pb.Host{}
		safescale := client.New().Host
		for _, i := range w.cluster.ListMasterIDs(w.feature.task) {
			host, err := safescale.Inspect(i, client.DefaultExecutionTimeout)
			if err != nil {
				return nil, err
			}
			w.allMasters = append(w.allMasters, host)
		}
	}
	return w.allMasters, nil
}

// identifyConcernedNodes returns a list of all the hosts acting nodes and keep this list
// during all the install session
func (w *worker) identifyConcernedNodes() ([]*pb.Host, error) {
	if w.cluster == nil {
		return []*pb.Host{}, nil
	}

	if w.concernedNodes == nil {
		hosts, err := w.identifyAllNodes()
		if err != nil {
			return nil, err
		}
		concernedHosts, err := w.extractHostsFailingCheck(hosts)
		if err != nil {
			return nil, err
		}
		w.concernedNodes = concernedHosts
	}
	return w.concernedNodes, nil
}

// identifyAllNodes returns a list of all the hosts acting as public of private nodes and keep this list
// during all the install session
func (w *worker) identifyAllNodes() ([]*pb.Host, error) {
	if w.cluster == nil {
		return []*pb.Host{}, nil
	}

	if w.allNodes == nil {
		hostClt := client.New().Host
		allHosts := []*pb.Host{}
		for _, i := range w.cluster.ListNodeIDs(w.feature.task) {
			host, err := hostClt.Inspect(i, client.DefaultExecutionTimeout)
			if err != nil {
				return nil, err
			}
			allHosts = append(allHosts, host)
		}
		w.allNodes = allHosts
	}
	return w.allNodes, nil
}

// identifyConcernedGateways returns a list of all the hosts acting as gateway that can accept the action
//  and keep this list during all the install session
func (w *worker) identifyConcernedGateways() ([]*pb.Host, error) {
	var hosts []*pb.Host

	if w.host != nil {
		host := gatewayFromHost(w.host)
		hosts = []*pb.Host{host}
	} else if w.cluster != nil {
		var err error
		hosts, err = w.identifyAllGateways()
		if err != nil {
			return nil, err
		}
	}

	concernedHosts, err := w.extractHostsFailingCheck(hosts)
	if err != nil {
		return nil, err
	}
	w.concernedGateways = concernedHosts
	return w.concernedGateways, nil
}

// identifyAllGateways returns a list of all the hosts acting as gatewaysand keep this list
// during all the install session
// For now, it's exactly the same than identifyAvailableGateway(), there is only one gateway authorized
func (w *worker) identifyAllGateways() ([]*pb.Host, error) {
	host, err := w.identifyAvailableGateway()
	if err != nil {
		return nil, err
	}
	return []*pb.Host{host}, nil
}

// Proceed executes the action
func (w *worker) Proceed(v Variables, s Settings) (Results, error) {
	w.variables = v
	w.settings = s

	results := Results{}

	// 'pace' tells the order of execution
	pace := w.feature.specs.GetString(w.rootKey + "." + yamlPaceKeyword)
	if pace == "" {
		return nil, fmt.Errorf("missing or empty key %s.%s", w.rootKey, yamlPaceKeyword)
	}

	// 'steps' describes the steps of the action
	stepsKey := w.rootKey + "." + yamlStepsKeyword
	steps := w.feature.specs.GetStringMap(stepsKey)
	if len(steps) <= 0 {
		return nil, fmt.Errorf("nothing to do")
	}
	order := strings.Split(pace, ",")

	// Applies reverseproxy rules to make it functional (feature may need it during the install)
	if w.action == Action.Add && !s.SkipProxy {
		err := w.setReverseProxy()
		if err != nil {
			return nil, err
		}
	}

	// Now enumerate steps and execute each of them
	var err error
	for _, k := range order {
		log.Debugf("executing step '%s::%s'...\n", w.action.String(), k)

		stepKey := stepsKey + "." + k
		var (
			runContent string
			stepT      = stepTargets{}
			options    = map[string]string{}
			ok         bool
			anon       interface{}
			err        error
		)
		stepMap, ok := steps[strings.ToLower(k)].(map[string]interface{})
		if !ok {
			msg := `syntax error in feature '%s' specification file (%s): no key '%s' found`
			return nil, fmt.Errorf(msg, w.feature.DisplayName(), w.feature.DisplayFilename(), stepKey)
		}

		// Determine list of hosts concerned by the step
		var hostsList []*pb.Host
		if w.target.Type() == "node" {
			hostsList, err = w.identifyHosts(map[string]string{"hosts": "1"})
		} else {
			anon, ok = stepMap[yamlTargetsKeyword]
			if ok {
				for i, j := range anon.(map[string]interface{}) {
					switch j.(type) {
					case bool:
						if j.(bool) {
							stepT[i] = "true"
						} else {
							stepT[i] = "false"
						}
					case string:
						stepT[i] = j.(string)
					}
				}
			} else {
				msg := `syntax error in feature '%s' specification file (%s): no key '%s.%s' found`
				return nil, fmt.Errorf(msg, w.feature.DisplayName(), w.feature.DisplayFilename(), stepKey, yamlTargetsKeyword)
			}

			hostsList, err = w.identifyHosts(stepT)
		}
		if err != nil {
			return nil, err
		}
		if len(hostsList) == 0 {
			continue
		}

		// Get the content of the action based on method
		keyword := yamlRunKeyword
		switch w.method {
		case Method.Apt:
			fallthrough
		case Method.Yum:
			fallthrough
		case Method.Dnf:
			keyword = yamlPackageKeyword
		}
		anon, ok = stepMap[keyword]
		if ok {
			runContent = anon.(string)
			// If 'run' content has to be altered, do it
			if w.commandCB != nil {
				runContent = w.commandCB(runContent)
			}
		} else {
			msg := `syntax error in feature '%s' specification file (%s): no key '%s.%s' found`
			return nil, fmt.Errorf(msg, w.feature.DisplayName(), w.feature.DisplayFilename(), stepKey, yamlRunKeyword)
		}

		// If there is an options file (for now specific to DCOS), upload it to the remote host
		optionsFileContent := ""
		anon, ok = stepMap[yamlOptionsKeyword]
		if ok {
			for i, j := range anon.(map[string]interface{}) {
				options[i] = j.(string)
			}
			var (
				avails  = map[string]interface{}{}
				ok      bool
				content interface{}
			)
			complexity := strings.ToLower(w.cluster.GetIdentity(w.feature.task).Complexity.String())
			for k, anon := range options {
				avails[strings.ToLower(k)] = anon
			}
			if content, ok = avails[complexity]; !ok {
				if complexity == strings.ToLower(Complexity.Large.String()) {
					complexity = Complexity.Normal.String()
				}
				if complexity == strings.ToLower(Complexity.Normal.String()) {
					if content, ok = avails[complexity]; !ok {
						content, ok = avails[Complexity.Small.String()]
					}
				}
			}
			if ok {
				optionsFileContent = content.(string)
				v["options"] = fmt.Sprintf("--options=%s/options.json", srvutils.TempFolder)
			}
		} else {
			v["options"] = ""
		}

		wallTime := 0
		anon, ok = stepMap[yamlTimeoutKeyword]
		if ok {
			if _, ok := anon.(int); ok {
				wallTime = anon.(int)
			} else {
				wallTime, err = strconv.Atoi(anon.(string))
				if err != nil {
					log.Warningf("Invalid value '%s' for '%s.%s', ignored.", anon.(string), w.rootKey, yamlTimeoutKeyword)
				}
			}
		}
		if wallTime == 0 {
			wallTime = 5
		}

		templateCommand, err := normalizeScript(Variables{
			"reserved_Name":    w.feature.DisplayName(),
			"reserved_Content": runContent,
			"reserved_Action":  strings.ToLower(w.action.String()),
			"reserved_Step":    k,
		})
		if err != nil {
			return nil, err
		}

		// Checks if step can be performed in parallel on selected hosts
		serial := false
		anon, ok = stepMap[yamlSerialKeyword]
		if ok {
			value, ok := anon.(string)
			if ok {
				if strings.ToLower(value) == "yes" || strings.ToLower(value) != "true" {
					serial = true
				}
			}
		}

		step := step{
			Worker:             w,
			Name:               k,
			Action:             w.action,
			Targets:            stepT,
			Script:             templateCommand,
			WallTime:           time.Duration(wallTime) * time.Minute,
			OptionsFileContent: optionsFileContent,
			YamlKey:            stepKey,
			Serial:             serial,
		}
		results[k], err = step.Run(hostsList, w.variables, w.settings)
		// If an error occured, don't do the remaining steps, fail immediately
		if err != nil {
			break
		}
		if !results[k].Successful() {
			break
		}
	}
	return results, err
}

// validateContextForCluster checks if the flavor of the cluster is listed in feature specification
// 'feature.suitableFor.cluster'.
// If no flavors is listed, no flavors are authorized (but using 'cluster: no' is strongly recommanded)
func (w *worker) validateContextForCluster() error {
	clusterFlavor := w.cluster.GetIdentity(w.feature.task).Flavor

	yamlKey := "feature.suitableFor.cluster"
	if w.feature.specs.IsSet(yamlKey) {
		flavors := strings.Split(w.feature.specs.GetString(yamlKey), ",")
		for _, k := range flavors {
			k = strings.ToLower(k)
			e, err := Flavor.Parse(k)
			if (err == nil && clusterFlavor == e) || (err != nil && k == "all") {
				return nil
			}
		}
	}
	msg := fmt.Sprintf("feature '%s' not suitable for flavor '%s' of cluster", w.feature.DisplayName(), clusterFlavor.String())
	return fmt.Errorf(msg)
}

// validateContextForHost ...
func (w *worker) validateContextForHost() error {
	if w.node {
		return nil
	}
	ok := false
	yamlKey := "feature.suitableFor.host"
	if w.feature.specs.IsSet(yamlKey) {
		value := strings.ToLower(w.feature.specs.GetString(yamlKey))
		ok = value == "ok" || value == "yes" || value == "true" || value == "1"
	}
	if ok {
		return nil
	}
	msg := fmt.Sprintf("feature '%s' not suitable for host", w.feature.DisplayName())
	// log.Println(msg)
	return fmt.Errorf(msg)
}

func (w *worker) validateClusterSizing() error {
	yamlKey := "feature.requirements.clusterSizing." + strings.ToLower(w.cluster.GetIdentity(w.feature.task).Flavor.String())
	if !w.feature.specs.IsSet(yamlKey) {
		return nil
	}
	sizing := w.feature.specs.GetStringMap(yamlKey)
	if anon, ok := sizing["minMasters"]; ok {
		minMasters := anon.(int)
		curMasters := len(w.cluster.ListMasterIDs(w.feature.task))
		if curMasters < minMasters {
			return fmt.Errorf("cluster doesn't meet the minimum number of masters (%d < %d)", curMasters, minMasters)
		}
	}
	if anon, ok := sizing["minNodes"]; ok {
		minNodes := anon.(int)
		curNodes := len(w.cluster.ListNodeIDs(w.feature.task))
		if curNodes < minNodes {
			return fmt.Errorf("cluster doesn't meet the minimum number of nodes (%d < %d)", curNodes, minNodes)
		}
	}

	return nil
}

// setReverseProxy applies the reverse proxy rules defined in specification file (if there are some)
func (w *worker) setReverseProxy() error {
	rules, ok := w.feature.specs.Get("feature.proxy.rules").([]interface{})
	if !ok || len(rules) <= 0 {
		return nil
	}

	var (
		err error
		gw  *pb.Host
	)

	gw, err = w.identifyAvailableGateway()
	if err != nil {
		return fmt.Errorf("failed to set reverse proxy: %s", err.Error())

	}

	kc, err := NewKongController(gw)
	if err != nil {
		return fmt.Errorf("failed to apply reverse proxy rules: %s", err.Error())
	}

	// Sets the values useable in any cases
	w.variables["PublicIP"] = gw.PublicIp
	w.variables["GatewayIP"] = gw.PrivateIp

	// Now submits all the rules to reverse proxy
	for _, r := range rules {
		targets := stepTargets{}
		rule := r.(map[interface{}]interface{})
		anon, ok := rule["targets"].(map[interface{}]interface{})
		if !ok {
			// If no 'targets' key found, applies on host only
			if w.cluster != nil {
				continue
			}
			targets[targetHosts] = "true"
		} else {
			for i, j := range anon {
				switch j.(type) {
				case bool:
					if j.(bool) {
						targets[i.(string)] = "true"
					} else {
						targets[i.(string)] = "false"
					}
				case string:
					targets[i.(string)] = j.(string)
				}
			}
		}
		hosts, err := w.identifyHosts(targets)
		if err != nil {
			return fmt.Errorf("failed to apply proxy rules: %s", err.Error())
		}
		for _, h := range hosts {
			w.variables["HostIP"] = h.PrivateIp
			w.variables["Hostname"] = h.Name
			err := kc.Apply(rule, &(w.variables))
			if err != nil {
				return fmt.Errorf("failed to apply proxy rules: %s", err.Error())
			}
		}
	}
	return nil
}

// identifyHosts identifies hosts concerned based on 'targets' and returns a list of hosts
func (w *worker) identifyHosts(targets stepTargets) ([]*pb.Host, error) {
	hostT, masterT, nodeT, gwT, err := targets.parse()
	if err != nil {
		return nil, err
	}

	var (
		hostsList = []*pb.Host{}
		all       []*pb.Host
	)

	if w.cluster == nil {
		if hostT != "" {
			hostsList = append(hostsList, w.host)
		}
		return hostsList, nil
	}

	switch masterT {
	case "1":
		host, err := w.identifyAvailableMaster()
		if err != nil {
			return nil, err
		}
		hostsList = append(hostsList, host)
	case "*":
		if w.action == Action.Add {
			all, err = w.identifyConcernedMasters()
		} else {
			all, err = w.identifyAllMasters()
		}
		if err != nil {
			return nil, err
		}
		hostsList = append(hostsList, all...)
	}

	switch nodeT {
	case "1":
		host, err := w.identifyAvailableNode()
		if err != nil {
			return nil, err
		}
		hostsList = append(hostsList, host)
	case "*":
		if w.action == Action.Add {
			all, err = w.identifyConcernedNodes()
		} else {
			all, err = w.identifyAllNodes()
		}
		if err != nil {
			return nil, err
		}
		hostsList = append(hostsList, all...)
	}

	switch gwT {
	case "1":
		host, err := w.identifyAvailableGateway()
		if err != nil {
			return nil, err
		}
		hostsList = append(hostsList, host)
	case "*":
		if w.action == Action.Add {
			all, err = w.identifyConcernedGateways()
		} else {
			all, err = w.identifyAllGateways()
		}
		if err != nil {
			return nil, err
		}
		hostsList = append(hostsList, all...)
	}
	return hostsList, nil
}