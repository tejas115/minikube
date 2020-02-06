/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package none

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/state"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	knet "k8s.io/apimachinery/pkg/util/net"
	pkgdrivers "k8s.io/minikube/pkg/drivers"
	"k8s.io/minikube/pkg/minikube/bootstrapper/bsutil/kverify"
	"k8s.io/minikube/pkg/minikube/command"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/cruntime"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
	"k8s.io/minikube/pkg/minikube/vmpath"
	"k8s.io/minikube/pkg/util/retry"
)

// cleanupPaths are paths to be removed by cleanup, and are used by both kubeadm and minikube.
var cleanupPaths = []string{
	vmpath.GuestEphemeralDir,
	vmpath.GuestManifestsDir,
	"/var/lib/minikube",
}

// Driver is a driver designed to run kubeadm w/o VM management, and assumes systemctl.
// https://minikube.sigs.k8s.io/docs/reference/drivers/none/
type Driver struct {
	*drivers.BaseDriver
	*pkgdrivers.CommonDriver
	URL     string
	runtime cruntime.Manager
	exec    command.Runner
}

// Config is configuration for the None driver
type Config struct {
	MachineName      string
	StorePath        string
	ContainerRuntime string
}

// NewDriver returns a fully configured None driver
func NewDriver(c Config) *Driver {
	runner := command.NewExecRunner()
	runtime, err := cruntime.New(cruntime.Config{Type: c.ContainerRuntime, Runner: runner})
	// Libraries shouldn't panic, but there is no way for drivers to return error :(
	if err != nil {
		glog.Fatalf("unable to create container runtime: %v", err)
	}
	return &Driver{
		BaseDriver: &drivers.BaseDriver{
			MachineName: c.MachineName,
			StorePath:   c.StorePath,
		},
		runtime: runtime,
		exec:    runner,
	}
}

// PreCreateCheck checks for correct privileges and dependencies
func (d *Driver) PreCreateCheck() error {
	return d.runtime.Available()
}

// Create a host using the driver's config
func (d *Driver) Create() error {
	// creation for the none driver is handled by commands.go
	return nil
}

// DriverName returns the name of the driver
func (d *Driver) DriverName() string {
	return "none"
}

// GetIP returns an IP or hostname that this host is available at
func (d *Driver) GetIP() (string, error) {
	ip, err := knet.ChooseHostInterface()
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

// GetSSHHostname returns hostname for use with ssh
func (d *Driver) GetSSHHostname() (string, error) {
	return "", fmt.Errorf("driver does not support ssh commands")
}

// GetSSHPort returns port for use with ssh
func (d *Driver) GetSSHPort() (int, error) {
	return 0, fmt.Errorf("driver does not support ssh commands")
}

// GetURL returns a Docker compatible host URL for connecting to this host
// e.g. tcp://1.2.3.4:2376
func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil
}

// GetState returns the state that the host is in (running, stopped, etc)
func (d *Driver) GetState() (state.State, error) {
	glog.Infof("GetState called")
	ip, err := d.GetIP()
	if err != nil {
		return state.Error, err
	}

	port, err := kubeconfig.Port(d.BaseDriver.MachineName)
	if err != nil {
		glog.Warningf("unable to get port: %v", err)
		port = constants.APIServerPort
	}

	// Confusing logic, as libmachine.Stop will loop until the state == Stopped
	ast, err := kverify.APIServerStatus(d.exec, net.ParseIP(ip), port)
	if err != nil {
		return ast, err
	}

	// If the apiserver is up, we'll claim to be up.
	if ast == state.Paused || ast == state.Running {
		return state.Running, nil
	}

	return kverify.KubeletStatus(d.exec)
}

// Kill stops a host forcefully, including any containers that we are managing.
func (d *Driver) Kill() error {
	if err := stopKubelet(d.exec); err != nil {
		return errors.Wrap(err, "kubelet")
	}

	// First try to gracefully stop containers
	containers, err := d.runtime.ListContainers(cruntime.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "containers")
	}
	if len(containers) == 0 {
		return nil
	}
	// Try to be graceful before sending SIGKILL everywhere.
	if err := d.runtime.StopContainers(containers); err != nil {
		return errors.Wrap(err, "stop")
	}

	containers, err = d.runtime.ListContainers(cruntime.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "containers")
	}
	if len(containers) == 0 {
		return nil
	}
	if err := d.runtime.KillContainers(containers); err != nil {
		return errors.Wrap(err, "kill")
	}
	return nil
}

// Remove a host, including any data which may have been written by it.
func (d *Driver) Remove() error {
	if err := d.Kill(); err != nil {
		return errors.Wrap(err, "kill")
	}
	glog.Infof("Removing: %s", cleanupPaths)
	args := append([]string{"rm", "-rf"}, cleanupPaths...)
	if _, err := d.exec.RunCmd(exec.Command("sudo", args...)); err != nil {
		glog.Errorf("cleanup incomplete: %v", err)
	}
	return nil
}

// Restart a host
func (d *Driver) Restart() error {
	return restartKubelet(d.exec)
}

// Start a host
func (d *Driver) Start() error {
	var err error
	d.IPAddress, err = d.GetIP()
	if err != nil {
		return err
	}
	d.URL, err = d.GetURL()
	if err != nil {
		return err
	}

	// Kill any processes that may interfere with kubeadm
	if err := stopKubelet(d.exec); err != nil {
		glog.Warningf("unable to stop kubelet: %v", err)
	}
	containers, err := d.runtime.ListContainers(cruntime.ListOptions{Namespaces: []string{"kube-system"}})
	if err != nil {
		glog.Warningf("unable to list kube-system containers: %v", err)
	}
	if len(containers) > 0 {
		glog.Warningf("found %d kube-system containers to stop", len(containers))
		if err := d.runtime.StopContainers(containers); err != nil {
			glog.Warningf("error stopping containers: %v", err)
		}
	}

	return nil
}

// Stop a host gracefully, including any containers that we are managing.
func (d *Driver) Stop() error {
	if err := stopKubelet(d.exec); err != nil {
		return errors.Wrap(err, "stop kubelet")
	}
	containers, err := d.runtime.ListContainers(cruntime.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "containers")
	}
	if len(containers) > 0 {
		if err := d.runtime.StopContainers(containers); err != nil {
			return errors.Wrap(err, "stop containers")
		}
	}
	glog.Infof("none driver is stopped!")
	return nil
}

// RunSSHCommandFromDriver implements direct ssh control to the driver
func (d *Driver) RunSSHCommandFromDriver() error {
	return fmt.Errorf("driver does not support ssh commands")
}

// stopKubelet idempotently stops the kubelet
func stopKubelet(cr command.Runner) error {
	glog.Infof("stopping kubelet.service ...")
	stop := func() error {
		cmd := exec.Command("sudo", "systemctl", "stop", "kubelet.service")
		if rr, err := cr.RunCmd(cmd); err != nil {
			glog.Errorf("temporary error for %q : %v", rr.Command(), err)
		}
		cmd = exec.Command("sudo", "systemctl", "show", "-p", "SubState", "kubelet")
		rr, err := cr.RunCmd(cmd)
		if err != nil {
			glog.Errorf("temporary error: for %q : %v", rr.Command(), err)
		}
		if !strings.Contains(rr.Stdout.String(), "dead") && !strings.Contains(rr.Stdout.String(), "failed") {
			return fmt.Errorf("unexpected kubelet state: %q", rr.Stdout.String())
		}
		return nil
	}

	if err := retry.Expo(stop, 2*time.Second, time.Minute*3, 5); err != nil {
		return errors.Wrapf(err, "error stopping kubelet")
	}

	return nil
}

// restartKubelet restarts the kubelet
func restartKubelet(cr command.Runner) error {
	glog.Infof("restarting kubelet.service ...")
	c := exec.Command("sudo", "systemctl", "restart", "kubelet.service")
	if _, err := cr.RunCmd(c); err != nil {
		return err
	}
	return nil
}
