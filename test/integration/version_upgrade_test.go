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

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/state"

	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/util/retry"

	"github.com/blang/semver"
	"github.com/hashicorp/go-getter"
	pkgutil "k8s.io/minikube/pkg/util"
)

// TestVersionUpgradeLatest is a comprehensive combination of tests
// downloads the current latest release binary from https://github.com/kubernetes/minikube/releases/latest
// and starts it with:  a custom ISO-URL and oldest supported k8s version
// and then stops it
// and then starts with head minikube binary with oldest supported k8s version
// and then starts with head minikube binary with newest supported k8s version
func TestVersionUpgradeLatest(t *testing.T) {
	MaybeParallel(t)
	profile := UniqueProfileName("vupgrade-latest")
	ctx, cancel := context.WithTimeout(context.Background(), Minutes(55))

	defer CleanupWithLogs(t, profile, cancel)

	tf, err := ioutil.TempFile("", "minikube-release.*.exe")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	defer os.Remove(tf.Name())
	tf.Close()

	url := pkgutil.GetBinaryDownloadURL("latest", runtime.GOOS)
	if err := retry.Expo(func() error { return getter.GetFile(tf.Name(), url) }, 3*time.Second, Minutes(3)); err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tf.Name(), 0700); err != nil {
			t.Errorf("chmod: %v", err)
		}
	}

	// Assert that --iso-url works without a sha checksum, and that we can upgrade from old ISO's
	// Some day, this will break an implicit assumption that a tool is available in the ISO :)
	oldISO := "https://storage.googleapis.com/minikube/iso/integration-test.iso"
	args := append([]string{"start", "-p", profile, "--memory=2200", fmt.Sprintf("--iso-url=%s", oldISO), fmt.Sprintf("--kubernetes-version=%s", constants.OldestKubernetesVersion), "--alsologtostderr", "-v=1"}, StartArgs()...)
	rr := &RunResult{}
	r := func() error {
		rr, err = Run(t, exec.CommandContext(ctx, tf.Name(), args...))
		return err
	}

	// Retry to allow flakiness for the previous release
	if err := retry.Expo(r, 1*time.Second, Minutes(30), 3); err != nil {
		t.Fatalf("release start failed: %v", err)
	}

	rr, err = Run(t, exec.CommandContext(ctx, tf.Name(), "stop", "-p", profile))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}

	rr, err = Run(t, exec.CommandContext(ctx, tf.Name(), "-p", profile, "status", "--format={{.Host}}"))
	if err != nil {
		t.Logf("status error: %v (may be ok)", err)
	}
	got := strings.TrimSpace(rr.Stdout.String())
	if got != state.Stopped.String() {
		t.Errorf("status = %q; want = %q", got, state.Stopped.String())
	}

	args = append([]string{"start", "-p", profile, fmt.Sprintf("--kubernetes-version=%s", constants.NewestKubernetesVersion), "--alsologtostderr", "-v=1"}, StartArgs()...)
	rr, err = Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("%s failed: %v", rr.Args, err)
	}

	s, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "version", "--output=json"))
	if err != nil {
		t.Fatalf("error running kubectl: %v", err)
	}
	cv := struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}{}
	err = json.Unmarshal(s.Stdout.Bytes(), &cv)

	if err != nil {
		t.Fatalf("error traversing json output: %v", err)
	}

	if cv.ServerVersion.GitVersion != constants.NewestKubernetesVersion {
		t.Fatalf("expected server version %s is not the same with latest version %s", cv.ServerVersion.GitVersion, constants.NewestKubernetesVersion)
	}

	args = append([]string{"start", "-p", profile, fmt.Sprintf("--kubernetes-version=%s", constants.OldestKubernetesVersion), "--alsologtostderr", "-v=1"}, StartArgs()...)
	rr = &RunResult{}
	r = func() error {
		rr, err = Run(t, exec.CommandContext(ctx, tf.Name(), args...))
		return err
	}

	if err := retry.Expo(r, 1*time.Second, Minutes(30), 3); err == nil {
		t.Fatalf("downgrading kubernetes should not be allowed: %v", err)
	}

	args = append([]string{"start", "-p", profile, fmt.Sprintf("--kubernetes-version=%s", constants.NewestKubernetesVersion), "--alsologtostderr", "-v=1"}, StartArgs()...)
	rr, err = Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("%s failed: %v", rr.Args, err)
	}
}

// TestVersionUpgradeV1
// downloads the minikube v1.0.0 binary and starts it
// stops it
// and then starts with head minikube binary
func TestVersionUpgradeV1(t *testing.T) {
	MaybeParallel(t)
	profile := UniqueProfileName("vupgrade-v1")
	ctx, cancel := context.WithTimeout(context.Background(), Minutes(55))

	defer CleanupWithLogs(t, profile, cancel)

	tf, err := ioutil.TempFile("", "minikube-v1-release.*.exe")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	defer os.Remove(tf.Name())
	tf.Close()

	url := pkgutil.GetBinaryDownloadURL("v1.0.0", runtime.GOOS)
	if err := retry.Expo(func() error { return getter.GetFile(tf.Name(), url) }, 3*time.Second, Minutes(3)); err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tf.Name(), 0700); err != nil {
			t.Errorf("chmod: %v", err)
		}
	}

	args := append([]string{"start", "-p", profile}, StartArgs()...)
	rr := &RunResult{}
	r := func() error {
		rr, err = Run(t, exec.CommandContext(ctx, tf.Name(), args...))
		return err
	}

	// Retry to allow flakiness for the previous release
	if err := retry.Expo(r, 13*time.Second, Minutes(30), 2); err != nil {
		t.Fatalf("v1.0.0 start failed: %v", err)
	}
	rr, err = Run(t, exec.CommandContext(ctx, tf.Name(), "stop", "-p", profile))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}

	rr, err = Run(t, exec.CommandContext(ctx, tf.Name(), "-p", profile, "status", "--format={{.Host}}"))
	if err != nil {
		t.Logf("status error: %v (may be ok)", err)
	}
	got := strings.TrimSpace(rr.Stdout.String())
	if got != state.Stopped.String() {
		t.Errorf("status = %q; want = %q", got, state.Stopped.String())
	}

	args = append([]string{"start", "-p", profile, "--alsologtostderr", "-v=1"}, StartArgsOld()...)
	rr, err = Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("%s failed: %v", rr.Args, err)
	}

	s, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "version", "--output=json"))
	if err != nil {
		t.Fatalf("error running kubectl: %v", err)
	}
	cv := struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}{}
	err = json.Unmarshal(s.Stdout.Bytes(), &cv)

	if err != nil {
		t.Fatalf("error traversing json output: %v", err)
	}
	if _, err = semver.Parse(cv.ServerVersion.GitVersion); err != nil {
		t.Fatalf("err parsing the kubernets version %v", err)
	}
}
