/*
Copyright 2019 The Kubernetes Authors All rights reserved.

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

package docker

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/blang/semver"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/golang/glog"
	"k8s.io/minikube/pkg/drivers/kic"
	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/registry"
)

func init() {
	if err := registry.Register(registry.DriverDef{
		Name:     driver.Docker,
		Config:   configure,
		Init:     func() drivers.Driver { return kic.NewDriver(kic.Config{OCIBinary: oci.Docker}) },
		Status:   status,
		Priority: registry.Experimental,
	}); err != nil {
		panic(fmt.Sprintf("register failed: %v", err))
	}
}

func configure(mc config.MachineConfig) (interface{}, error) {
	return kic.NewDriver(kic.Config{
		MachineName:   mc.Name,
		StorePath:     localpath.MiniPath(),
		ImageDigest:   kic.BaseImage,
		CPU:           mc.CPUs,
		Memory:        mc.Memory,
		OCIBinary:     oci.Docker,
		APIServerPort: mc.Nodes[0].Port,
	}), nil
}

func status() registry.State {
	_, err := exec.LookPath(oci.Docker)
	if err != nil {
		return registry.State{Error: err, Installed: false, Healthy: false, Fix: "Docker is required.", Doc: "https://minikube.sigs.k8s.io/docs/reference/drivers/docker/"}
	}

	cmd := exec.Command(oci.Docker, "version", "-f", "'{{.Server.Version}}'")
	o, err := cmd.CombinedOutput()
	output := string(o)
	if err != nil {
		return registry.State{Error: err, Installed: true, Healthy: false, Fix: "Cant verify minimum required version for podman . See podman website for installation guide.", Doc: "https://docs.docker.com/"}
	}

	v, err := semver.Make(output)
	if v.LT(constants.MinSuggestDockerVer) {
		glog.Warningf("Warning ! minimum suggested version for docker is %s. your version is %q. minikube might not work. use at your own risk. To install a more recent version please see https://docs.docker.com/", constants.MinSuggestDockerVer.String(), v.String())
	}
	if v.LT(constants.MinReqDockerVer) {
		return registry.State{Error: err, Installed: true, Healthy: false, Fix: fmt.Sprintf("Your docker version is too old (%s) please the minimum required docker version is %s.", v.String(), constants.MinReqDockerVer.String()), Doc: "https://docs.docker.com/"}
	}

	// Allow no more than 3 seconds for querying state
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = exec.CommandContext(ctx, "docker", "info").Run()
	if err != nil {
		return registry.State{Error: err, Installed: true, Healthy: false, Fix: "Docker is not running or is responding too slow. Try: restarting docker desktop."}
	}

	return registry.State{Installed: true, Healthy: true}
}
