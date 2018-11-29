#!/bin/bash

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# This script downloads the test files from the build bucket and makes some executable.

# The script expects the following env variables:
# OS_ARCH: The operating system and the architecture separated by a hyphen '-' (e.g. darwin-amd64, linux-amd64, windows-amd64)
# VM_DRIVER: the vm-driver to use for the test
# EXTRA_START_ARGS: additional flags to pass into minikube start
# EXTRA_ARGS: additional flags to pass into minikube
# JOB_NAME: the name of the logfile and check name to update on github
#


readonly TEST_ROOT="${HOME}/minikube-integration"
readonly TEST_HOME="${TEST_ROOT}/${OS_ARCH}-${VM_DRIVER}-${MINIKUBE_LOCATION}-$$-${COMMIT}"
echo ">> Starting at $(date)"
echo ""
echo "arch:      ${OS_ARCH}"
echo "build:     ${MINIKUBE_LOCATION}"
echo "driver:    ${VM_DRIVER}"
echo "job:       ${JOB_NAME}"
echo "test home: ${TEST_HOME}"
echo "sudo:      ${SUDO_PREFIX}"
echo "kernel:    $(uname -v)"
# Setting KUBECONFIG prevents the version ceck from erroring out due to permission issues
echo "kubectl:   $(env KUBECONFIG=${TEST_HOME} kubectl version --client --short=true)"
echo "docker:    $(docker version --format '{{ .Client.Version }}')"

case "${VM_DRIVER}" in
  kvm2)
    echo "virsh:     $(virsh --version)"
  ;;
  virtualbox)
    echo "vbox:      $(vboxmanage --version)"
  ;;
esac

echo ""
mkdir -p out/ testdata/ "${TEST_HOME}"

# Install gsutil if necessary.
if ! type -P gsutil >/dev/null; then
  if [[ ! -x "out/gsutil/gsutil" ]]; then
    echo "Installing gsutil to $(pwd)/out ..."
    curl -s https://storage.googleapis.com/pub/gsutil.tar.gz | tar -C out/ -zxf -
  fi
  PATH="$(pwd)/out/gsutil:$PATH"
fi

# Add the out/ directory to the PATH, for using new drivers.
PATH="$(pwd)/out/":$PATH
export PATH

echo ""
echo ">> Downloading test inputs from ${MINIKUBE_LOCATION} ..."
gsutil -qm cp \
  "gs://minikube-builds/${MINIKUBE_LOCATION}/minikube-${OS_ARCH}" \
  "gs://minikube-builds/${MINIKUBE_LOCATION}/docker-machine-driver"-* \
  "gs://minikube-builds/${MINIKUBE_LOCATION}/e2e-${OS_ARCH}" out

gsutil -qm cp "gs://minikube-builds/${MINIKUBE_LOCATION}/testdata"/* testdata/

# Set the executable bit on the e2e binary and out binary
export MINIKUBE_BIN="out/minikube-${OS_ARCH}"
export E2E_BIN="out/e2e-${OS_ARCH}"
chmod +x "${MINIKUBE_BIN}" "${E2E_BIN}" out/docker-machine-driver-*

echo ""
echo ">> Cleaning up after previous test runs ..."

# Cleanup stale test outputs.
pgrep minikube && echo "WARNING: other instances of minikube may be running"

for stale_dir in "${TEST_ROOT}"/*; do
  echo "* Cleaning stale test: ${stale_dir}"
  export MINIKUBE_HOME="${stale_dir}/.minikube"
  export KUBECONFIG="${stale_dir}/kubeconfig"

  if [[ -d "${MINIKUBE_HOME}" ]]; then
    echo "Shutting down stale minikube instance ..."
    if [[ -w "${MINIKUBE_HOME}" ]]; then
        "${MINIKUBE_BIN}" delete || true
        rm -Rf "${MINIKUBE_HOME}"
    else
      sudo -E "${MINIKUBE_BIN}" delete || true
      sudo rm -Rf "${MINIKUBE_HOME}"
    fi
  fi

  if [[ -f "${KUBECONFIG}" ]]; then
    if [[ -w "${KUBECONFIG}" ]]; then
      rm -f "${KUBECONFIG}"
    else
      sudo rm -f "${KUBECONFIG}" || true
    fi
  fi

  rmdir "${stale_dir}" || true
done

if type -P virsh; then
  virsh -c qemu:///system list --all
  virsh -c qemu:///system list --all \
    | grep minikube \
    | awk '{ print $2 }' \
    | xargs -I {} sh -c "virsh -c qemu:///system destroy {}; virsh -c qemu:///system undefine {}"
fi

if type -P vboxmanage; then
  vboxmanage list vms
  vboxmanage list vms \
    | cut -d'"' -f2 \
    | xargs -I {} sh -c "vboxmanage startvm {} --type emergencystop; vboxmanage unregistervm {} --delete" \
    || true
fi

if type -P hdiutil; then
  hdiutil info | grep -E "/dev/disk[1-9][^s]"
  hdiutil info \
      | grep -E "/dev/disk[1-9][^s]" \
      | awk '{print $1}' \
      | xargs -I {} sh -c "hdiutil detach {}" \
      || true
fi

if [[ "${OS_ARCH}" == "Darwin" ]]; then
  pgrep xhyve | xargs kill || true
  if [[ -e out/docker-machine-driver-hyperkit ]]; then
    sudo chown root:wheel out/docker-machine-driver-hyperkit || true
   sudo chmod u+s out/docker-machine-driver-hyperkit || true
  fi
fi

if pgrep kubectl; then
  echo "stale kubectl processes found: $(pgrep kubectl)"
  pgrep kubectl | xargs kill
fi

# Respected by minikube
export MINIKUBE_HOME="${TEST_HOME}/.minikube"
export MINIKUBE_WANTREPORTERRORPROMPT=False

# Respected by kubectl
export KUBECONFIG="${TEST_HOME}/kubeconfig"

# Display the default image URL
echo ""
echo ">> ISO URL"
"${MINIKUBE_BIN}" start -h | grep iso-url

echo ""
echo ">> Starting ${E2E_BIN} at $(date)"
${SUDO_PREFIX}${E2E_BIN} \
  -minikube-start-args="--vm-driver=${VM_DRIVER} ${EXTRA_START_ARGS}" \
  -minikube-args="--v=10 --logtostderr ${EXTRA_ARGS}" \
  -test.v -test.timeout=30m -binary="${MINIKUBE_BIN}" && result=$? || result=$?
echo ">> ${E2E_BIN} exited with ${result} at $(date)"
echo ""

if [[ $result -eq 0 ]]; then
  status="success"
  echo "minikube: SUCCESS"
else
  status="failure"
  echo "minikube: FAIL"
  source print-debug-info.sh
fi

echo ">> Cleaning up after ourselves ..."
${SUDO_PREFIX}${MINIKUBE_BIN} delete >/dev/null 2>/dev/null || true
${SUDO_PREFIX} rm -Rf "${MINIKUBE_HOME}"
${SUDO_PREFIX} rm -f "${KUBECONFIG}"
rmdir "${TEST_HOME}"
echo ">> ${TEST_HOME} completed at $(date)"

readonly target_url="https://storage.googleapis.com/minikube-builds/logs/${MINIKUBE_LOCATION}/${JOB_NAME}.txt"
curl "https://api.github.com/repos/kubernetes/minikube/statuses/${COMMIT}?access_token=$access_token" \
  -H "Content-Type: application/json" \
  -X POST \
  -d "{\"state\": \"$status\", \"description\": \"Jenkins\", \"target_url\": \"$target_url\", \"context\": \"${JOB_NAME}\"}"

exit $result