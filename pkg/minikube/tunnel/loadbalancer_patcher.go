/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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

package tunnel

import (
	"fmt"

	"github.com/golang/glog"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type requestSender interface {
	send(request *rest.Request) (result []byte, err error)
}

type patchConverter interface {
	convert(restClient rest.Interface, patch *Patch) *rest.Request
}

type loadBalancerEmulator struct {
	coreV1Client   v1.CoreV1Interface
	requestSender  requestSender
	patchConverter patchConverter
}

func (l *loadBalancerEmulator) PatchServices() (managedServices []string, err error) {
	managedServices, err = l.applyOnLBServices(func(restClient rest.Interface, svc core_v1.Service) ([]byte, error) {
		return l.updateService(restClient, svc)
	})
	return
}

func (l *loadBalancerEmulator) Cleanup() (managedServices []string, err error) {
	managedServices, err = l.applyOnLBServices(func(restClient rest.Interface, svc core_v1.Service) ([]byte, error) {
		return l.cleanupService(restClient, svc)
	})
	return
}

func (l *loadBalancerEmulator) applyOnLBServices(action func(restClient rest.Interface, svc core_v1.Service) ([]byte, error)) ([]string, error) {
	services := l.coreV1Client.Services("")
	serviceList, e := services.List(metav1.ListOptions{})
	if e != nil {
		return nil, e
	}
	restClient := l.coreV1Client.RESTClient()

	var managedServices []string

	for _, svc := range serviceList.Items {
		if svc.Spec.Type == "LoadBalancer" {
			glog.Infof("%s is type LoadBalancer.", svc.Name)
			managedServices = append(managedServices, svc.Name)
			result, err := action(restClient, svc)
			if err != nil {
				glog.Errorf("%s", result)
				glog.Errorf("error patching service %s/%s: %s", svc.Namespace, svc.Name, err)
				continue
			}
		} else {
			glog.V(3).Infof("%s is not type LoadBalancer, skipping.", svc.Name)
		}
	}
	return managedServices, nil
}
func (l *loadBalancerEmulator) updateService(restClient rest.Interface, svc core_v1.Service) (result []byte, err error) {
	clusterIP := svc.Spec.ClusterIP
	ingresses := svc.Status.LoadBalancer.Ingress
	if len(ingresses) == 0 || (len(ingresses) == 1 && ingresses[0].IP != clusterIP) {
		glog.V(3).Infof("[%s] setting ClusterIP as the LoadBalancer Ingress", svc.Name)
		jsonPatch := fmt.Sprintf(`[{"op": "add", "path": "/status/loadBalancer/ingress", "value":  [ { "ip": "%s" } ] }]`, clusterIP)
		patch := &Patch{
			Type:         k8s_types.JSONPatchType,
			ResourceName: svc.Name,
			NameSpaceSet: true,
			NameSpace:    svc.Namespace,
			Subresource:  "status",
			Resource:     "services",
			BodyContent:  jsonPatch,
		}
		request := l.patchConverter.convert(restClient, patch)
		result, err = l.requestSender.send(request)
		glog.Infof("Patched %s with IP %s", svc.Name, clusterIP)
		return
	}
	return nil, nil
}

func (l *loadBalancerEmulator) cleanupService(restClient rest.Interface, svc core_v1.Service) (result []byte, err error) {
	ingresses := svc.Status.LoadBalancer.Ingress
	if len(ingresses) > 0 {
		glog.V(3).Infof("[%s] cleanup: unset load balancer ingress", svc.Name)
		jsonPatch := `[{"op": "remove", "path": "/status/loadBalancer/ingress" }]`
		patch := &Patch{
			Type:         k8s_types.JSONPatchType,
			ResourceName: svc.Name,
			NameSpaceSet: true,
			NameSpace:    svc.Namespace,
			Subresource:  "status",
			Resource:     "services",
			BodyContent:  jsonPatch,
		}
		request := l.patchConverter.convert(restClient, patch)
		result, err = l.requestSender.send(request)
		glog.Infof("Removed load balancer ingress from %s.", svc.Name)
		return
	}
	return nil, nil
}

func NewLoadBalancerEmulator(corev1Client v1.CoreV1Interface) *loadBalancerEmulator {
	return &loadBalancerEmulator{
		coreV1Client:   corev1Client,
		requestSender:  &defaultRequestSender{},
		patchConverter: &defaultPatchConverter{},
	}
}

type defaultPatchConverter struct{}

func (c *defaultPatchConverter) convert(restClient rest.Interface, patch *Patch) (request *rest.Request) {
	request = restClient.Patch(patch.Type)
	request.Name(patch.ResourceName)
	request.Resource(patch.Resource)
	request.SubResource(patch.Subresource)
	if patch.NameSpaceSet {
		request.Namespace(patch.NameSpace)
	}
	request.Body([]byte(patch.BodyContent))
	return
}

type defaultRequestSender struct{}

func (r *defaultRequestSender) send(request *rest.Request) (result []byte, err error) {
	return request.Do().Raw()
}
