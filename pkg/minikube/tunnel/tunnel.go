/*
Copyright 2018 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tunnel

import (
	"fmt"
	"github.com/docker/machine/libmachine"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/minikube/pkg/minikube/config"
	"os"
)

type tunnel interface {
	cleanup() *TunnelStatus
	updateTunnelStatus() *TunnelStatus
}

func errorTunnelAlreadyExists(id *TunnelID) error {
	return fmt.Errorf("there is already a running tunnel for this machine: %s", id)
}

func newTunnel(machineName string,
	machineAPI libmachine.API,
	configLoader config.ConfigLoader,
	v1Core v1.CoreV1Interface, registry *persistentRegistry, router router) (*minikubeTunnel, error) {
	clusterInspector := &minikubeInspector{
		machineName:  machineName,
		machineAPI:   machineAPI,
		configLoader: configLoader,
	}
	state, route, err := clusterInspector.getStateAndRoute()
	//TODO(balintp): clean this up to be more self contained
	machineAPI.Close()
	if err != nil {
		return nil, fmt.Errorf("unable to determine cluster info: %s", err)
	}
	id := TunnelID{
		Route:       route,
		MachineName: machineName,
		Pid:         getPid(),
	}
	runningTunnel, err := registry.IsAlreadyDefinedAndRunning(&id)
	if err != nil {
		return nil, fmt.Errorf("unable to check tunnel registry for conflict: %s", err)
	}
	if runningTunnel != nil {
		return nil, fmt.Errorf("another tunnel is already running, shut it down first: %s", runningTunnel)
	}

	return &minikubeTunnel{
		clusterInspector:     clusterInspector,
		router:               router,
		registry:             registry,
		loadBalancerEmulator: NewLoadBalancerEmulator(v1Core),
		status: &TunnelStatus{
			TunnelID:      id,
			MinikubeState: state,
		},
		reporter: &simpleReporter{
			out: os.Stdout,
		},
	}, nil

}

type minikubeTunnel struct {
	//collaborators
	clusterInspector     *minikubeInspector
	router               router
	loadBalancerEmulator *loadBalancerEmulator
	reporter             reporter
	registry             *persistentRegistry

	status *TunnelStatus
}

func (t *minikubeTunnel) cleanup() *TunnelStatus {
	glog.V(3).Infof("cleaning up %s", t.status.TunnelID.Route)
	e := t.router.Cleanup(t.status.TunnelID.Route)
	if e != nil {
		t.status.RouteError = errors.Errorf("error cleaning up route: %s", e)
		glog.V(3).Infof(t.status.RouteError.Error())
	} else {
		t.registry.Remove(t.status.TunnelID.Route)
	}
	if t.status.MinikubeState == Running {
		t.status.PatchedServices, t.status.LoadBalancerEmulatorError = t.loadBalancerEmulator.Cleanup()
	}
	return t.status
}

func (t *minikubeTunnel) updateTunnelStatus() *TunnelStatus {
	glog.V(3).Info("updating tunnel status...")
	t.status.MinikubeState, _, t.status.MinikubeError = t.clusterInspector.getStateAndHost()
	//TODO(balintp): clean this up to be more self contained
	defer t.clusterInspector.machineAPI.Close()
	if t.status.MinikubeState == Running {
		glog.V(3).Infof("minikube is running, trying to add Route %s", t.status.TunnelID.Route)

		exists, conflict, _, err := t.router.Inspect(t.status.TunnelID.Route)
		if err != nil {
			t.status.RouteError = fmt.Errorf("error checking for route state: %s", err)
		} else if !exists && len(conflict) == 0 {
			t.status.RouteError = t.router.EnsureRouteIsAdded(t.status.TunnelID.Route)
			if t.status.RouteError == nil {
				//the route was added successfully, we need to make sure the registry has it too
				//this might fail in race conditions, when another process created this tunnel
				if err := t.registry.Register(&t.status.TunnelID); err != nil {
					glog.Errorf("failed to register tunnel: %s", err)
					t.status.RouteError = err
				}
			}
		} else if len(conflict) > 0 {
			t.status.RouteError = fmt.Errorf("conflicting route: %s", conflict)
		} else {
			//the route exists, make sure that this process owns it in the registry
			conflictingTunnel, e := t.registry.IsAlreadyDefinedAndRunning(&t.status.TunnelID)
			if e != nil {
				glog.Errorf("failed to check for other tunnels: %s", e)
				t.status.RouteError = e
			}
			if conflictingTunnel == nil {
				//the route exists, but "orphaned", this process will "own it" in the registry
				if err := t.registry.Register(&t.status.TunnelID); err != nil {
					glog.Errorf("failed to register tunnel: %s", err)
					t.status.RouteError = err
				}
			} else if conflictingTunnel.Pid != getPid() {
				//another process owns the tunnel
				t.status.RouteError = errorTunnelAlreadyExists(conflictingTunnel)
			}

		}
		if t.status.RouteError == nil {
			t.status.PatchedServices, t.status.LoadBalancerEmulatorError = t.loadBalancerEmulator.PatchServices()
		}
	}
	glog.V(3).Infof("sending report %s", t.status)
	t.reporter.Report(t.status.Clone())
	return t.status
}
