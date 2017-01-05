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
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	commonutil "k8s.io/minikube/pkg/util"
	"k8s.io/minikube/test/integration/util"
)

func testClusterDNS(t *testing.T) {
	t.Parallel()
	kubectlRunner := util.NewKubectlRunner(t)
	podName := "e2e"
	podPath, _ := filepath.Abs("testdata/e2e.yaml")

	dnsTest := func() error {
		podNamespace := kubectlRunner.CreateRandomNamespace()
		defer kubectlRunner.DeleteNamespace(podNamespace)

		if _, err := kubectlRunner.RunCommand([]string{"create", "-f", podPath, "--namespace=" + podNamespace}); err != nil {
			return err
		}
		defer kubectlRunner.RunCommand([]string{"delete", "-f", podPath, "--namespace=" + podNamespace})

		p := &api.Pod{}
		for p.Status.Phase != "Running" {
			p = kubectlRunner.GetPod(podName, podNamespace)
		}

		dnsByteArr, err := kubectlRunner.RunCommand([]string{"exec", podName, "--namespace=" + podNamespace,
			"dns"})
		dnsOutput := string(dnsByteArr)
		if err != nil {
			return &commonutil.RetriableError{Err: err}
		}

		if !strings.Contains(dnsOutput, "10.0.0.1") {
			return fmt.Errorf("DNS lookup failed, could not find 10.0.0.1. Output: %s", dnsOutput)
		}
		return nil
	}

	if err := commonutil.RetryAfter(40, dnsTest, 5*time.Second); err != nil {
		t.Fatalf("DNS lookup failed with error:", err)
	}
}
