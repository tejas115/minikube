/*
Copyright 2020 The Kubernetes Authors All rights reserved.

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

package cni

import (
	"bytes"
	"context"
	"text/template"

	"k8s.io/minikube/pkg/minikube/assets"
	"k8s.io/minikube/pkg/minikube/config"
)

// bridge is what minikube defaulted to when `--enable-default-cni=true`
// https://github.com/containernetworking/plugins/blob/master/plugins/main/bridge/README.md

var bridgeConf = template.Must(template.New("bridge").Parse(`
{
  "cniVersion": "0.3.1",
  "name": "bridge",
  "type": "bridge",
  "bridge": "bridge",
  "addIf": "true",
  "isDefaultGateway": true,
  "forceAddress": false,
  "ipMasq": true,
  "hairpinMode": true,
  "ipam": {
      "type": "host-local",
      "subnet": "{{.PodCIDR}}"
  }
}
`))

// Bridge is a CNI manager than does nothing
type Bridge struct {
	cc config.ClusterConfig
}

// Assets returns a list of assets necessary to enable this CNI
func (n Bridge) Assets() ([]assets.CopyableFile, error) {
	input := &tmplInput{PodCIDR: defaultPodCIDR}

	b := bytes.Buffer{}
	if err := bridgeConf.Execute(&b, input); err != nil {
		return nil, err
	}

	return []assets.CopyableFile{assets.NewMemoryAssetTarget(b.Bytes(), "/etc/cni/net.d/1-k8s.conf", "0644")}, nil
}

// NeedsApply returns whether or not CNI requires a manifest to be applied
func (n Bridge) NeedsApply() bool {
	return false
}

// Apply enables the CNI
func (n Bridge) Apply(context.Context, Runner) error {
	return nil
}

// CIDR returns the default CIDR used by this CNI
func (n Bridge) CIDR() string {
	return defaultPodCIDR
}
