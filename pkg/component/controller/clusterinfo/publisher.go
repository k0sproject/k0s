// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterinfo

import (
	"context"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const StackName = "cluster-info"

// Publisher publishes the cluster-info and what joining nodes read, while api.clusterInfoDiscovery is enabled.
type Publisher struct {
	K0sVars *config.CfgVars
	// The node-local configuration, for the API, the network, the cluster domain and DNS.
	NodeConfig *v1beta1.ClusterConfig
	Clients    kubeutil.ClientFactoryInterface
}

var _ manager.Component = (*Publisher)(nil)

// Applies the stack to the cluster. An empty stack removes what an earlier start applied.
func (p *Publisher) Init(ctx context.Context) error {
	var resources []*unstructured.Unstructured
	if p.NodeConfig.Spec.API.ClusterInfoDiscovery {
		endpoint, err := p.NodeConfig.Spec.API.APIServerHostPort()
		if err != nil {
			return err
		}
		ca, err := os.ReadFile(filepath.Join(p.K0sVars.CertRootDir, "ca.crt"))
		if err != nil {
			return err
		}
		network := p.NodeConfig.Spec.Network
		family := p.NodeConfig.Spec.PrimaryAddressFamily()
		dnsAddress, err := network.DNSAddress(family)
		if err != nil {
			return err
		}
		kubernetesVersion := constant.KubeProxyImageVersion
		if build.KubernetesVersion != "" {
			kubernetesVersion = "v" + build.KubernetesVersion
		}
		objects, err := Render(&Input{
			ControlPlaneEndpoint: endpoint.String(),
			CACertPEM:            ca,
			KubernetesVersion:    kubernetesVersion,
			ServiceCIDR:          network.BuildServiceCIDR(family),
			PodCIDR:              network.BuildPodCIDR(family),
			ClusterDomain:        network.ClusterDomain,
			DNSAddress:           dnsAddress,
		})
		if err != nil {
			return err
		}
		resources, err = applier.ToUnstructuredSlice(nil, objects...)
		if err != nil {
			return err
		}
	}
	return applier.ApplyStack(ctx, p.Clients, resources, StackName)
}

func (p *Publisher) Start(context.Context) error { return nil }
func (p *Publisher) Stop() error                 { return nil }
