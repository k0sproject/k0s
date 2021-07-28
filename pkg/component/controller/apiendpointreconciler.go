/*
Copyright 2020 k0s authors

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
package controller

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sort"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// APIEndpointReconciler is the component to reconcile in-cluster API address endpoint based from externalName
type APIEndpointReconciler struct {
	ClusterConfig *v1beta1.ClusterConfig

	L *logrus.Entry

	leaderElector     LeaderElector
	stopCh            chan struct{}
	kubeClientFactory k8sutil.ClientFactory
}

// NewEndpointReconciler creates new endpoint reconciler
func NewEndpointReconciler(c *v1beta1.ClusterConfig, leaderElector LeaderElector, kubeClientFactory k8sutil.ClientFactory) *APIEndpointReconciler {
	d := atomic.Value{}
	d.Store(true)
	return &APIEndpointReconciler{
		ClusterConfig:     c,
		leaderElector:     leaderElector,
		stopCh:            make(chan struct{}),
		kubeClientFactory: kubeClientFactory,
		L:                 logrus.WithFields(logrus.Fields{"component": "endpointreconciler"}),
	}
}

// Init initializes the APIEndpointReconciler
func (a *APIEndpointReconciler) Init() error {
	_, err := net.LookupIP(a.ClusterConfig.Spec.API.ExternalAddress)
	if err != nil {
		return fmt.Errorf("cannot resolve api.externalAddress: %w", err)
	}

	return nil
}

// Run runs the main loop for reconciling the externalAddress
func (a *APIEndpointReconciler) Run() error {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := a.reconcileEndpoints()
				if err != nil {
					a.L.Warnf("external API address reconciliation failed: %s", err.Error())
				}
			case <-a.stopCh:
				a.L.Info("endpoint reconciler done")
				return
			}
		}
	}()

	return nil
}

// Stop stops the reconciler
func (a *APIEndpointReconciler) Stop() error {
	close(a.stopCh)
	return nil
}

// Healthy dummy implementation
func (a *APIEndpointReconciler) Healthy() error { return nil }

func (a *APIEndpointReconciler) reconcileEndpoints() error {
	if !a.leaderElector.IsLeader() {
		a.L.Debug("we're not the leader, not reconciling api endpoints")
		return nil
	}

	ips, err := net.LookupIP(a.ClusterConfig.Spec.API.ExternalAddress)
	if err != nil {
		a.L.Errorf("cannot resolve api.externalAddress: %s", err.Error())
		return err
	}
	// Sort the addresses so we can more easily tell if we need to update the endpoints or not
	ipStrings := make([]string, len(ips))
	for i, ip := range ips {
		ipStrings[i] = ip.String()
	}
	sort.Strings(ipStrings)

	c, err := a.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	epClient := c.CoreV1().Endpoints("default")

	ep, err := epClient.Get(context.TODO(), "kubernetes", v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			err := a.createEndpoint(ipStrings)
			return err
		}

		return err
	}

	if len(ep.Subsets) == 0 || needsUpdate(ipStrings, ep) {
		ep.Subsets = []corev1.EndpointSubset{
			corev1.EndpointSubset{
				Addresses: stringsToEndpointAddresses(ipStrings),
				Ports: []corev1.EndpointPort{
					corev1.EndpointPort{
						Name:     "https",
						Protocol: "TCP",
						Port:     int32(a.ClusterConfig.Spec.API.Port),
					},
				},
			},
		}

		_, err := epClient.Update(context.TODO(), ep, v1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *APIEndpointReconciler) createEndpoint(addresses []string) error {
	ep := &corev1.Endpoints{
		TypeMeta: v1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "kubernetes",
		},
		Subsets: []corev1.EndpointSubset{
			corev1.EndpointSubset{
				Addresses: stringsToEndpointAddresses(addresses),
				Ports: []corev1.EndpointPort{
					corev1.EndpointPort{
						Name:     "https",
						Protocol: "TCP",
						Port:     int32(a.ClusterConfig.Spec.API.Port),
					},
				},
			},
		},
	}

	c, err := a.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	_, err = c.CoreV1().Endpoints("default").Create(context.TODO(), ep, v1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func needsUpdate(newAddresses []string, ep *corev1.Endpoints) bool {
	currentAddresses := endpointAddressesToStrings(ep.Subsets[0].Addresses)
	sort.Strings(currentAddresses)
	return reflect.DeepEqual(currentAddresses, newAddresses)
}

func endpointAddressesToStrings(eps []corev1.EndpointAddress) []string {
	a := make([]string, len(eps))

	for i, e := range eps {
		a[i] = e.IP
	}

	return a
}

func stringsToEndpointAddresses(addresses []string) []corev1.EndpointAddress {
	eps := make([]corev1.EndpointAddress, len(addresses))

	for i, a := range addresses {
		eps[i] = corev1.EndpointAddress{
			IP: a,
		}
	}

	return eps
}
