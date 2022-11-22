/*
Copyright 2022 k0s authors

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
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type TunneledEndpointReconciler struct {
	cfg *v1beta1.ClusterConfig

	logger *logrus.Entry

	leaderElector     leaderelector.Interface
	kubeClientFactory k8sutil.ClientFactoryInterface
}

func (ter TunneledEndpointReconciler) Init(_ context.Context) error {
	return nil
}

func (ter *TunneledEndpointReconciler) Start(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := ter.reconcile(ctx)
				if err != nil {
					ter.logger.Warnf("external API address reconciliation failed: %s", err.Error())
				}
			case <-ctx.Done():
				ter.logger.Info("endpoint reconciler done")
				return
			}
		}
	}()
	return nil
}

func (ter *TunneledEndpointReconciler) Stop() error {
	return nil
}

func (ter *TunneledEndpointReconciler) Reconcile(ctx context.Context, cfg *v1beta1.ClusterConfig) error {
	ter.cfg = cfg
	return nil
}

func (ter TunneledEndpointReconciler) reconcile(ctx context.Context) error {
	if ter.cfg == nil {
		return nil
	}
	if !ter.leaderElector.IsLeader() {
		ter.logger.Debug("we're not the leader, not reconciling api endpoints")
		return nil
	}

	if !ter.cfg.Spec.API.TunneledNetworkingMode {
		return fmt.Errorf("impossible to disable tunneled networking for the KAS server on the fly, restart the node")
	}

	if err := ter.makeDefaultServiceInternalOnly(ctx); err != nil {
		return fmt.Errorf("can't make `kubernetes` service be internal only: %w", err)
	}

	if err := ter.reconcileEndpoint(ctx); err != nil {
		return fmt.Errorf("can't reconcile endpoint for the default service: %w", err)
	}
	return nil
}

func (ter TunneledEndpointReconciler) reconcileEndpoint(ctx context.Context) error {
	c, err := ter.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	epClient := c.CoreV1().Endpoints("default")

	addresses, err := makeNodesAddresses(ctx, c)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return nil
	}
	subsets := []v1core.EndpointSubset{
		{
			Addresses: addresses,
			Ports: []v1core.EndpointPort{
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     6443,
				},
			},
		},
	}
	kubernetesEndpoint, err := epClient.Get(ctx, "kubernetes", v1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			return ter.createEndpoint(ctx, subsets)
		}
		return err
	}

	kubernetesEndpoint.Subsets = subsets
	_, err = epClient.Update(ctx, kubernetesEndpoint, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func makeNodesAddresses(ctx context.Context, c kubernetes.Interface) ([]v1core.EndpointAddress, error) {
	nodes, err := c.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't list nodes: %w", err)
	}

	addresses := make([]v1core.EndpointAddress, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		var publicAddr string
		var internalAddr string
		node := node
		for _, addr := range node.Status.Addresses {
			switch addr.Type {
			case v1core.NodeInternalIP:
				internalAddr = addr.Address
			case v1core.NodeExternalIP:
				publicAddr = addr.Address
			}
		}
		if publicAddr == "" && internalAddr == "" {
			continue
		}

		// try use internal address, if not found fallback to public
		address := internalAddr
		if address == "" {
			address = publicAddr
		}
		addresses = append(addresses, v1core.EndpointAddress{
			IP:       address,
			NodeName: &node.Name,
		})
	}
	return addresses, nil
}

func (ter TunneledEndpointReconciler) createEndpoint(ctx context.Context, subsets []v1core.EndpointSubset) error {

	ep := &v1core.Endpoints{
		TypeMeta: v1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "kubernetes",
		},
		Subsets: subsets,
	}

	c, err := ter.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	_, err = c.CoreV1().Endpoints("default").Create(ctx, ep, v1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("can't create new endpoints for kubernetes serice: %w", err)
	}

	return nil
}

func (ter TunneledEndpointReconciler) makeDefaultServiceInternalOnly(ctx context.Context) error {
	c, err := ter.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}

	svcClient := c.CoreV1().Services("default")

	svc, err := svcClient.Get(ctx, "kubernetes", v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get default service: %w", err)
	}

	newSvc := svc.DeepCopy()
	p := v1core.ServiceInternalTrafficPolicyLocal
	newSvc.Spec.InternalTrafficPolicy = &p

	if _, err := svcClient.Update(ctx, newSvc, v1.UpdateOptions{}); err != nil {
		return fmt.Errorf("can't update default service: %w", err)
	}
	return nil
}

func NewTunneledEndpointReconciler(leaderElector leaderelector.Interface, kubeClientFactory k8sutil.ClientFactoryInterface) *TunneledEndpointReconciler {
	return &TunneledEndpointReconciler{
		leaderElector:     leaderElector,
		kubeClientFactory: kubeClientFactory,
		logger:            logrus.WithFields(logrus.Fields{"component": "tunneled_endpoint_reconciler"}),
	}
}
