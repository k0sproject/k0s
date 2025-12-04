// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sort"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/sirupsen/logrus"
)

// Dummy checks so we catch easily if we miss some interface implementation
var _ manager.Component = (*APIEndpointReconciler)(nil)

type resolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

// APIEndpointReconciler is the component to reconcile in-cluster API address endpoint based from externalName
type APIEndpointReconciler struct {
	logger *logrus.Entry

	externalAddress   string
	apiServerPort     int
	leaderStatus      func() leaderelection.Status
	kubeClientFactory kubeutil.ClientFactoryInterface
	resolver          resolver
	afnet             string

	stopCh chan struct{}
}

// NewEndpointReconciler creates new endpoint reconciler
func NewEndpointReconciler(nodeConfig *v1beta1.ClusterConfig, leaderStatus func() leaderelection.Status, kubeClientFactory kubeutil.ClientFactoryInterface, resolver resolver, primaryAddressFamily v1beta1.PrimaryAddressFamilyType) *APIEndpointReconciler {
	var afnet string
	switch primaryAddressFamily {
	case v1beta1.PrimaryFamilyIPv4:
		afnet = "ip4"
	case v1beta1.PrimaryFamilyIPv6:
		afnet = "ip6"
	}

	return &APIEndpointReconciler{
		logger:            logrus.WithFields(logrus.Fields{"component": "endpointreconciler"}),
		externalAddress:   nodeConfig.Spec.API.ExternalHost(),
		apiServerPort:     nodeConfig.Spec.API.ExternalPort(),
		leaderStatus:      leaderStatus,
		stopCh:            make(chan struct{}),
		kubeClientFactory: kubeClientFactory,
		resolver:          resolver,
		afnet:             afnet,
	}
}

// Init initializes the APIEndpointReconciler
func (a *APIEndpointReconciler) Init(_ context.Context) error {
	return nil
}

// Run runs the main loop for reconciling the externalAddress
func (a *APIEndpointReconciler) Start(ctx context.Context) error {

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := a.reconcileEndpoints(ctx)
				if err != nil {
					a.logger.WithError(err).Warn("External API address reconciliation failed")
				}
			case <-a.stopCh:
				a.logger.Info("Endpoint reconciler done")
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

func (a *APIEndpointReconciler) reconcileEndpoints(ctx context.Context) error {
	if a.leaderStatus() != leaderelection.StatusLeading {
		a.logger.Debug("Not the leader, not reconciling API endpoints")
		return nil
	}

	externalAddress := a.externalAddress

	ipAddrs, err := a.resolver.LookupIP(ctx, a.afnet, externalAddress)
	if err != nil {
		return fmt.Errorf("while resolving external address %q: %w", externalAddress, err)
	}

	// Sort the addresses so we can more easily tell if we need to update the endpoints or not
	ipStrings := make([]string, len(ipAddrs))
	for i, ipAddr := range ipAddrs {
		ipStrings[i] = ipAddr.String()
	}
	sort.Strings(ipStrings)

	c, err := a.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	epClient := c.CoreV1().Endpoints(metav1.NamespaceDefault)

	ep, err := epClient.Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		if err := a.createEndpoint(ctx, ipStrings); err != nil {
			return fmt.Errorf("failed to create Endpoints: %w", err)
		}

		a.logger.Debugf("Successfully created Endpoints resource using %v", ipStrings)
		return nil
	}

	if len(ep.Subsets) == 0 || needsUpdate(ipStrings, ep) {
		ep.Subsets = []corev1.EndpointSubset{{
			Addresses: stringsToEndpointAddresses(ipStrings),
			Ports: []corev1.EndpointPort{
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     int32(a.apiServerPort),
				},
			},
		}}

		_, err := epClient.Update(ctx, ep, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update Endpoints: %w", err)
		}

		a.logger.Debugf("Successfully updated Endpoints resource using %v", ipStrings)
	}

	return nil
}

func (a *APIEndpointReconciler) createEndpoint(ctx context.Context, addresses []string) error {
	ep := &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes",
		},
		Subsets: []corev1.EndpointSubset{{
			Addresses: stringsToEndpointAddresses(addresses),
			Ports: []corev1.EndpointPort{
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     int32(a.apiServerPort),
				},
			},
		}},
	}

	c, err := a.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	_, err = c.CoreV1().Endpoints(metav1.NamespaceDefault).Create(ctx, ep, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func needsUpdate(newAddresses []string, ep *corev1.Endpoints) bool {
	currentAddresses := endpointAddressesToStrings(ep.Subsets[0].Addresses)
	sort.Strings(currentAddresses)
	return !reflect.DeepEqual(currentAddresses, newAddresses)
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
