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
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/golang/glog"
	"k8s.io/minikube/pkg/drivers/kic"
	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/registry"
)

var docURL = "https://minikube.sigs.k8s.io/docs/drivers/docker/"

func init() {
	if err := registry.Register(registry.DriverDef{
		Name:     driver.Docker,
		Config:   configure,
		Init:     func() drivers.Driver { return kic.NewDriver(kic.Config{OCIBinary: oci.Docker}) },
		Status:   status,
		Priority: registry.HighlyPreferred,
	}); err != nil {
		panic(fmt.Sprintf("register failed: %v", err))
	}
}

func configure(cc config.ClusterConfig, n config.Node) (interface{}, error) {
	mounts := make([]oci.Mount, len(cc.ContainerVolumeMounts))
	for i, spec := range cc.ContainerVolumeMounts {
		var err error
		mounts[i], err = oci.ParseMountString(spec)
		if err != nil {
			return nil, err
		}
	}

	return kic.NewDriver(kic.Config{
		MachineName:       driver.MachineName(cc, n),
		StorePath:         localpath.MiniPath(),
		ImageDigest:       cc.KicBaseImage,
		Mounts:            mounts,
		CPU:               cc.CPUs,
		Memory:            cc.Memory,
		OCIBinary:         oci.Docker,
		APIServerPort:     cc.Nodes[0].Port,
		KubernetesVersion: cc.KubernetesConfig.KubernetesVersion,
		ContainerRuntime:  cc.KubernetesConfig.ContainerRuntime,
	}), nil
}

func status() registry.State {
	if runtime.GOARCH != "amd64" {
		return registry.State{Error: fmt.Errorf("docker driver is not supported on %q systems yet", runtime.GOARCH), Installed: false, Healthy: false, Fix: "Try other drivers", Doc: docURL}
	}

	_, err := exec.LookPath(oci.Docker)
	if err != nil {
		return registry.State{Error: err, Installed: false, Healthy: false, Fix: "Install Docker", Doc: docURL}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// Quickly returns an error code if server is not running
	cmd := exec.CommandContext(ctx, oci.Docker, "version", "--format", "{{.Server.Os}}-{{.Server.Version}}")
	o, err := cmd.Output()
	output := string(o)
	if strings.Contains(output, "windows-") {
		return registry.State{Error: oci.ErrWindowsContainers, Installed: true, Healthy: false, Fix: "Change container type to \"linux\" in Docker Desktop settings", Doc: docURL + "#verify-docker-container-type-is-linux"}

	}
	if err == nil {
		glog.Infof("docker version: %s", output)
		return checkNeedsImprovement()
	}

	glog.Warningf("docker returned error: %v", err)

	// Basic timeout
	if ctx.Err() == context.DeadlineExceeded {
		glog.Warningf("%q timed out. ", strings.Join(cmd.Args, " "))
		return registry.State{Error: err, Installed: true, Healthy: false, Fix: "Restart the Docker service", Doc: docURL}
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		newErr := fmt.Errorf(`%q %v: %s`, strings.Join(cmd.Args, " "), exitErr, stderr)
		return suggestFix(stderr, newErr)
	}

	return registry.State{Error: err, Installed: true, Healthy: false, Doc: docURL}
}

// checkNeedsImprovement if overlay mod is installed on a system
func checkNeedsImprovement() registry.State {
	if runtime.GOOS == "linux" {
		return checkOverlayMod()
	}
	return registry.State{Installed: true, Healthy: true}
}

// checkOverlayMod checks if
func checkOverlayMod() registry.State {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		glog.Infof("Skipping overlay check: WSL does not support kernel modules")
	}

	if _, err := os.Stat("/lib/modules"); err != nil {
		glog.Infof("Skipping overlay check: distribution does not support kernel modules")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "modprobe", "overlay")
	_, err := cmd.Output()
	if err != nil {
		// try a different way
		cmd = exec.CommandContext(ctx, "uname", "-r")
		out, err := cmd.Output()
		if err != nil {
			glog.Warningf("unable to validate kernel version: %s", err)
			return registry.State{Installed: true, Healthy: true}
		}

		path := fmt.Sprintf("/lib/modules/%s/modules.builtin", string(out))
		cmd = exec.CommandContext(ctx, "cat", path)
		out, err = cmd.Output()
		if err != nil {
			glog.Warningf("overlay module was not found in %q", path)
			return registry.State{NeedsImprovement: true, Installed: true, Healthy: true, Fix: "enable the overlay Linux kernel module using 'modprobe overlay'"}
		}
		if strings.Contains(string(out), "overlay") { // success
			return registry.State{NeedsImprovement: false, Installed: true, Healthy: true}
		}
		glog.Warningf("overlay module was not found")
		return registry.State{NeedsImprovement: true, Installed: true, Healthy: true}
	}
	return registry.State{Installed: true, Healthy: true}
}

// suggestFix matches a stderr with possible fix for the docker driver
func suggestFix(stderr string, err error) registry.State {
	if strings.Contains(stderr, "permission denied") && runtime.GOOS == "linux" {
		return registry.State{Error: err, Installed: true, Running: true, Healthy: false, Fix: "Add your user to the 'docker' group: 'sudo usermod -aG docker $USER && newgrp docker'", Doc: "https://docs.docker.com/engine/install/linux-postinstall/"}
	}

	if strings.Contains(stderr, "/pipe/docker_engine: The system cannot find the file specified.") && runtime.GOOS == "windows" {
		return registry.State{Error: err, Installed: true, Running: false, Healthy: false, Fix: "Start the Docker service. If Docker is already running, you may need to reset Docker to factory settings with: Settings > Reset.", Doc: "https://github.com/docker/for-win/issues/1825#issuecomment-450501157"}
	}

	if strings.Contains(stderr, "Cannot connect") || strings.Contains(stderr, "refused") || strings.Contains(stderr, "Is the docker daemon running") || strings.Contains(stderr, "docker daemon is not running") {
		return registry.State{Error: err, Installed: true, Running: false, Healthy: false, Fix: "Start the Docker service", Doc: docURL}
	}

	// We don't have good advice, but at least we can provide a good error message
	return registry.State{Error: err, Installed: true, Running: true, Healthy: false, Doc: docURL}
}
