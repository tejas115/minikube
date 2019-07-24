// +build integration

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

package integration

import (
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/state"
	"k8s.io/minikube/test/integration/util"
)

func TestPersistence(t *testing.T) {
	t.Parallel()
	p := t.Name() // profile name
	mk := NewMinikubeRunner(t, p, "--wait=false")
	if usingNoneDriver(mk) {
		t.Skip("skipping test as none driver does not support persistence")
	}
	mk.EnsureRunning()

	kr := util.NewKubectlRunner(t, p)
	curdir, err := filepath.Abs("")
	if err != nil {
		t.Errorf("Error getting the file path for current directory: %s", curdir)
	}
	podPath := path.Join(curdir, "testdata", "busybox.yaml")

	// Create a pod and wait for it to be running.
	if _, err := kr.RunCommand([]string{"create", "-f", podPath}); err != nil {
		t.Fatalf("Error creating test pod: %v", err)
	}

	verify := func(t *testing.T) {
		if err := util.WaitForBusyboxRunning(t, "default", p); err != nil {
			t.Fatalf("waiting for busybox to be up: %v", err)
		}

	}

	// Make sure everything is up before we stop.
	verify(t)

	// Now restart minikube and make sure the pod is still there.
	// mk.RunCommand("stop", true)
	// mk.CheckStatus("Stopped")
	checkStop := func() error {
		mk.RunCommand("stop", true)
		return mk.CheckStatusNoFail(state.Stopped.String())
	}

	if err := util.Retry(t, checkStop, 5*time.Second, 6); err != nil {
		t.Fatalf("timed out while checking stopped status: %v", err)
	}

	mk.Start()
	mk.CheckStatus(state.Running.String())

	// Make sure the same things come up after we've restarted.
	verify(t)
}
