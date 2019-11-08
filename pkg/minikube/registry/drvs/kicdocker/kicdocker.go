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

package kicdocker

import (
	"fmt"
	"os/exec"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/golang/glog"
	"github.com/medyagh/kic/pkg/image"
	"k8s.io/minikube/pkg/drivers/kic"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/registry"
)

func init() {
	if err := registry.Register(registry.DriverDef{
		Name:     driver.KicDocker,
		Config:   configure,
		Init:     func() drivers.Driver { return kic.NewDriver(kic.Config{}) },
		Status:   status,
		Priority: registry.Discouraged, // experimental
	}); err != nil {
		panic(fmt.Sprintf("register failed: %v", err))
	}
}

func configure(mc config.MachineConfig) interface{} {
	imgSha, err := image.NameForVersion(mc.KubeVersion)
	if err != nil {
		glog.Errorf("Failed to getting image name for %s: imgesha:%s", imgSha, mc.KubeVersion)
	}

	return kic.NewDriver(kic.Config{
		MachineName:   config.GetMachineName(),
		StorePath:     localpath.MiniPath(),
		ImageSha:      imgSha,
		CPU:           mc.CPUs,
		Memory:        mc.Memory,
		APIServerPort: 5013, // (medya dbg: todo generate or get from config)
		OciBinary:     "docker",
	})

}

func status() registry.State {
	_, err := exec.LookPath("docker")
	if err != nil {
		return registry.State{Error: err, Installed: false, Healthy: false, Fix: "Docker is required.", Doc: "https://minikube.sigs.k8s.io/docs/reference/drivers/kic/"}
	}

	err = exec.Command("docker", "info").Run()
	if err != nil {
		return registry.State{Error: err, Installed: true, Healthy: false, Fix: "Docker is not running. Try: restarting docker desktop."}
	}

	return registry.State{Installed: true, Healthy: true}
}
