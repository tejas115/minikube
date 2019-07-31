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

package util

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/minikube/pkg/minikube/assets"
	commonutil "k8s.io/minikube/pkg/util"
)

const kubectlBinary = "kubectl"

// MinikubeRunner runs a command
type MinikubeRunner struct {
	T          *testing.T
	BinaryPath string
	GlobalArgs string
	StartArgs  string
	MountArgs  string
	Runtime    string
}

// Logf writes logs to stdout if -v is set.
func Logf(str string, args ...interface{}) {
	if !testing.Verbose() {
		return
	}
	fmt.Printf(" %s | ", time.Now().Format("15:04:05"))
	fmt.Println(fmt.Sprintf(str, args...))
}

// Run executes a command
func (m *MinikubeRunner) Run(cmd string) error {
	_, err := m.SSH(cmd)
	return err
}

// Copy copies a file
func (m *MinikubeRunner) Copy(f assets.CopyableFile) error {
	path, _ := filepath.Abs(m.BinaryPath)
	cmd := exec.Command("/bin/bash", "-c", path, "ssh", "--", fmt.Sprintf("cat >> %s", filepath.Join(f.GetTargetDir(), f.GetTargetName())))
	Logf("Running: %s", cmd.Args)
	return cmd.Run()
}

// CombinedOutput executes a command, returning the combined stdout and stderr
func (m *MinikubeRunner) CombinedOutput(cmd string) (string, error) {
	return m.SSH(cmd)
}

// Remove removes a file
func (m *MinikubeRunner) Remove(f assets.CopyableFile) error {
	_, err := m.SSH(fmt.Sprintf("rm -rf %s", filepath.Join(f.GetTargetDir(), f.GetTargetName())))
	return err
}

// teeRun runs a command, streaming stdout, stderr to console
func (m *MinikubeRunner) teeRun(cmd *exec.Cmd) (string, string, error) {
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", err
	}
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}

	if err := cmd.Start(); err != nil {
		return "", "", err
	}
	var outB bytes.Buffer
	var errB bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		if err := commonutil.TeePrefix(commonutil.ErrPrefix, errPipe, &errB, Logf); err != nil {
			m.T.Logf("tee: %v", err)
		}
		wg.Done()
	}()
	go func() {
		if err := commonutil.TeePrefix(commonutil.OutPrefix, outPipe, &outB, Logf); err != nil {
			m.T.Logf("tee: %v", err)
		}
		wg.Done()
	}()
	err = cmd.Wait()
	wg.Wait()
	return outB.String(), errB.String(), err
}

// RunCommand executes a command, optionally checking for error
func (m *MinikubeRunner) RunCommand(cmdStr string, failError bool) string {
	profileArg := fmt.Sprintf("-p=%s ", m.Profile)
	cmdStr = profileArg + cmdStr
	cmdArgs := strings.Split(cmdStr, " ")
	path, _ := filepath.Abs(m.BinaryPath)

	cmd := exec.Command(path, cmdArgs...)
	Logf("Run: %s", cmd.Args)
	stdout, stderr, err := m.teeRun(cmd)
	if failError && err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			m.T.Fatalf("Error running command: %s %s. Output: %s", cmdStr, exitError.Stderr, stdout)
		} else {
			m.T.Fatalf("Error running command: %s %v. Output: %s", cmdStr, err, stderr)
		}
	}
	return stdout
}

// RunWithContext calls the minikube command with a context, useful for timeouts.
func (m *MinikubeRunner) RunWithContext(ctx context.Context, cmdStr string) (string, string, error) {
	profileArg := fmt.Sprintf("-p=%s ", m.Profile)
	cmdStr = profileArg + cmdStr
	cmdArgs := strings.Split(cmdStr, " ")
	path, _ := filepath.Abs(m.BinaryPath)

	cmd := exec.CommandContext(ctx, path, cmdArgs...)
	Logf("Run: %s", cmd.Args)
	return m.teeRun(cmd)
}

// RunDaemon executes a command, returning the stdout
func (m *MinikubeRunner) RunDaemon(cmdStr string) (*exec.Cmd, *bufio.Reader) {
	profileArg := fmt.Sprintf("-p=%s ", m.Profile)
	cmdStr = profileArg + cmdStr
	cmdArgs := strings.Split(cmdStr, " ")
	path, _ := filepath.Abs(m.BinaryPath)

	cmd := exec.Command(path, cmdArgs...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		m.T.Fatalf("stdout pipe failed: %s %v", cmdStr, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		m.T.Fatalf("stderr pipe failed: %s %v", cmdStr, err)
	}

	var errB bytes.Buffer
	go func() {
		if err := commonutil.TeePrefix(commonutil.ErrPrefix, stderrPipe, &errB, Logf); err != nil {
			m.T.Logf("tee: %v", err)
		}
	}()

	err = cmd.Start()
	if err != nil {
		m.T.Fatalf("Error running command: %s %v", cmdStr, err)
	}
	return cmd, bufio.NewReader(stdoutPipe)

}

// RunDaemon2 executes a command, returning the stdout and stderr
func (m *MinikubeRunner) RunDaemon2(cmdStr string) (*exec.Cmd, *bufio.Reader, *bufio.Reader) {
	profileArg := fmt.Sprintf("-p=%s ", m.Profile)
	cmdStr = profileArg + cmdStr
	cmdArgs := strings.Split(cmdStr, " ")
	path, _ := filepath.Abs(m.BinaryPath)
	cmd := exec.Command(path, cmdArgs...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		m.T.Fatalf("stdout pipe failed: %s %v", cmdStr, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		m.T.Fatalf("stderr pipe failed: %s %v", cmdStr, err)
	}

	err = cmd.Start()
	if err != nil {
		m.T.Fatalf("Error running command: %s %v", cmdStr, err)
	}
	return cmd, bufio.NewReader(stdoutPipe), bufio.NewReader(stderrPipe)
}

// SSH returns the output of running a command using SSH
func (m *MinikubeRunner) SSH(cmdStr string) (string, error) {
	profileArg := fmt.Sprintf(" -p=%s", m.Profile)
	cmdStr += profileArg

	path, _ := filepath.Abs(m.BinaryPath)

	cmd := exec.Command(path, profileArg, "ssh", cmdStr)
	Logf("SSH: %s", cmdStr)
	stdout, err := cmd.CombinedOutput()
	Logf("Output: %s", stdout)
	if err, ok := err.(*exec.ExitError); ok {
		return string(stdout), err
	}
	return string(stdout), nil
}

// Start starts the cluster
func (m *MinikubeRunner) Start(opts ...string) {
	cmd := fmt.Sprintf("start %s %s %s --alsologtostderr --v=2", m.StartArgs, m.GlobalArgs, strings.Join(opts, " "))
	m.RunCommand(cmd, true)
}

// EnsureRunning makes sure the container runtime is running
func (m *MinikubeRunner) EnsureRunning() {
	if m.GetStatus() != "Running" {
		m.Start()
	}
	m.CheckStatus("Running")
}

// ParseEnvCmdOutput parses the output of `env` (assumes bash)
func (m *MinikubeRunner) ParseEnvCmdOutput(out string) map[string]string {
	env := map[string]string{}
	re := regexp.MustCompile(`(\w+?) ?= ?"?(.+?)"?\n`)
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		env[m[1]] = m[2]
	}
	return env
}

// GetStatus returns the status of a service
func (m *MinikubeRunner) GetStatus() string {
	return m.RunCommand(fmt.Sprintf("status -p=%s --format={{.Host}} %s", m.Profile, m.GlobalArgs), false)
}

// GetLogs returns the logs of a service
func (m *MinikubeRunner) GetLogs() string {
	return m.RunCommand(fmt.Sprintf("logs -p=%s %s", m.Profile, m.GlobalArgs), true)
}

// CheckStatus makes sure the service has the desired status, or cause fatal error
func (m *MinikubeRunner) CheckStatus(desired string) {
	err := m.CheckStatusNoFail(desired)
	if err != nil && desired != state.None.String() { // none status returns 1 exit code
		m.T.Fatalf("%v", err)
	}
}

// CheckStatusNoFail makes sure the service has the desired status, returning error
func (m *MinikubeRunner) CheckStatusNoFail(desired string) error {
	s := m.GetStatus()
	if s != desired {
		return fmt.Errorf("got state: %q, expected %q", s, desired)
	}
	return nil
}

// WaitForBusyboxRunning waits until busybox pod to be running
func WaitForBusyboxRunning(t *testing.T, namespace string) error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}
	selector := labels.SelectorFromSet(labels.Set(map[string]string{"integration-test": "busybox"}))
	return commonutil.WaitForPodsWithLabelRunning(client, namespace, selector)
}

// WaitForIngressControllerRunning waits until ingress controller pod to be running
func WaitForIngressControllerRunning(t *testing.T) error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	if err := commonutil.WaitForDeploymentToStabilize(client, "kube-system", "nginx-ingress-controller", time.Minute*10); err != nil {
		return errors.Wrap(err, "waiting for ingress-controller deployment to stabilize")
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"app.kubernetes.io/name": "nginx-ingress-controller"}))
	if err := commonutil.WaitForPodsWithLabelRunning(client, "kube-system", selector); err != nil {
		return errors.Wrap(err, "waiting for ingress-controller pods")
	}

	return nil
}

// WaitForGvisorControllerRunning waits for the gvisor controller pod to be running
func WaitForGvisorControllerRunning(t *testing.T) error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"kubernetes.io/minikube-addons": "gvisor"}))
	if err := commonutil.WaitForPodsWithLabelRunning(client, "kube-system", selector); err != nil {
		return errors.Wrap(err, "waiting for gvisor controller pod to stabilize")
	}
	return nil
}

// WaitForGvisorControllerDeleted waits for the gvisor controller pod to be deleted
func WaitForGvisorControllerDeleted() error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"kubernetes.io/minikube-addons": "gvisor"}))
	if err := commonutil.WaitForPodDelete(client, "kube-system", selector); err != nil {
		return errors.Wrap(err, "waiting for gvisor controller pod deletion")
	}
	return nil
}

// WaitForUntrustedNginxRunning waits for the untrusted nginx pod to start running
func WaitForUntrustedNginxRunning() error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"run": "nginx"}))
	if err := commonutil.WaitForPodsWithLabelRunning(client, "default", selector); err != nil {
		return errors.Wrap(err, "waiting for nginx pods")
	}
	return nil
}

// WaitForFailedCreatePodSandBoxEvent waits for a FailedCreatePodSandBox event to appear
func WaitForFailedCreatePodSandBoxEvent() error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}
	if err := commonutil.WaitForEvent(client, "default", "FailedCreatePodSandBox"); err != nil {
		return errors.Wrap(err, "waiting for FailedCreatePodSandBox event")
	}
	return nil
}

// WaitForNginxRunning waits for nginx service to be up
func WaitForNginxRunning(t *testing.T) error {
	client, err := commonutil.GetClient()

	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"run": "nginx"}))
	if err := commonutil.WaitForPodsWithLabelRunning(client, "default", selector); err != nil {
		return errors.Wrap(err, "waiting for nginx pods")
	}

	if err := commonutil.WaitForService(client, "default", "nginx", true, time.Millisecond*500, time.Minute*10); err != nil {
		t.Errorf("Error waiting for nginx service to be up")
	}
	return nil
}

// Retry tries the callback for a number of attempts, with a delay between attempts
func Retry(t *testing.T, callback func() error, d time.Duration, attempts int) (err error) {
	for i := 0; i < attempts; i++ {
		err = callback()
		if err == nil {
			return nil
		}
		time.Sleep(d)
	}
	return err
}
