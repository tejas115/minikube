package cruntime

import (
	"fmt"

	"github.com/golang/glog"
)

// Docker contains Docker runtime state
type Docker struct {
	config Config
}

// Name is a human readable name for Docker
func (r *Docker) Name() string {
	return "Docker"
}

// SocketPath returns the path to the socket file for Docker
func (r *Docker) SocketPath() string {
	return ""
}

// Active returns if docker is active on the host
func (r *Docker) Active(cr CommandRunner) bool {
	err := cr.Run("systemctl is-active --quiet service docker")
	if err == nil {
		return true
	}
	return false
}

// Enable idempotently enables Docker on a host
func (r *Docker) Enable(cr CommandRunner) error {
	if err := disableOthers(r, cr); err != nil {
		glog.Warningf("disableOthers: %v", err)
	}
	return cr.Run("sudo systemctl restart docker")
}

// Disable idempotently disables Docker on a host
func (r *Docker) Disable(cr CommandRunner) error {
	return cr.Run("sudo systemctl stop docker docker.socket")
}

// LoadImage loads an image into this runtime
func (r *Docker) LoadImage(cr CommandRunner, path string) error {
	return cr.Run(fmt.Sprintf("docker load -i %s", path))
}

// KubeletOptions returns kubelet options for a runtime.
func (r *Docker) KubeletOptions() map[string]string {
	return map[string]string{
		"container-runtime": "docker",
	}
}
