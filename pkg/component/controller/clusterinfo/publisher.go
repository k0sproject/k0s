// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterinfo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component/controller/externalnodes"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
)

const (
	// The stack in the manifests directory.
	stackName = "cluster-info"
	// How often the manifest is refreshed.
	resyncPeriod = 30 * time.Second
)

// Publisher publishes the cluster-info and what joining nodes read, while api.clusterInfoDiscovery is enabled.
type Publisher struct {
	K0sVars *config.CfgVars
	// The node-local configuration, for the API, the service CIDR, the cluster domain and DNS.
	NodeConfig *v1beta1.ClusterConfig
	Clients    kubeutil.ClientFactoryInterface

	log         logrus.FieldLogger
	config      value.Latest[*v1beta1.ClusterConfig]
	stop        func()
	manifestDir string
	// Whether the warning about an endpoint that changes with the leader was logged.
	warnedLeaderAddress bool
}

var (
	_ manager.Component  = (*Publisher)(nil)
	_ manager.Reconciler = (*Publisher)(nil)
)

func (p *Publisher) Init(context.Context) error {
	p.log = logrus.WithField("component", stackName)
	// The stack only exists while enabled, so other clusters see no changes.
	p.manifestDir = filepath.Join(p.K0sVars.ManifestsDir, stackName)
	return nil
}

// Reconcile implements [manager.Reconciler].
func (p *Publisher) Reconcile(_ context.Context, cfg *v1beta1.ClusterConfig) error {
	p.config.Set(cfg.DeepCopy())
	return nil
}

func (p *Publisher) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(resyncPeriod)
		defer ticker.Stop()
		cfg, changed := p.config.Peek()
		for {
			if cfg != nil {
				if err := p.sync(ctx, cfg); err != nil {
					p.log.WithError(err).Error("Failed to update the manifests, retrying")
				}
			}
			select {
			case <-changed:
				cfg, changed = p.config.Peek()
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
	p.stop = func() { cancel(); <-done }
	return nil
}

func (p *Publisher) Stop() error {
	if p.stop != nil {
		p.stop()
	}
	return nil
}

func (p *Publisher) sync(ctx context.Context, cfg *v1beta1.ClusterConfig) error {
	if !p.NodeConfig.Spec.API.ClusterInfoDiscovery {
		return os.RemoveAll(p.manifestDir)
	}
	endpoint, err := externalnodes.ControlPlaneEndpoint(p.NodeConfig.Spec)
	if err != nil {
		return err
	}
	p.warnOnLeaderAddress(ctx)
	in, err := p.input(cfg, endpoint)
	if err != nil {
		return err
	}
	objects, err := Render(in)
	if err != nil {
		return err
	}
	err = dir.Init(p.manifestDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	return applier.WriteManifest(filepath.Join(p.manifestDir, "cluster-info.yaml"), objects)
}

// warnOnLeaderAddress warns once if the published endpoint is the address of the leading controller in an HA setup.
// That address changes with the leader, and joined nodes keep using the old one.
func (p *Publisher) warnOnLeaderAddress(ctx context.Context) {
	if !externalnodes.UsesLeaderAddress(p.NodeConfig.Spec) {
		return
	}
	client, err := p.Clients.GetClient()
	if err != nil {
		return
	}
	endpointSlices, err := client.DiscoveryV1().EndpointSlices(metav1.NamespaceDefault).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=kubernetes",
	})
	if err != nil {
		return
	}
	var controllers int
	for i := range endpointSlices.Items {
		controllers += len(endpointSlices.Items[i].Endpoints)
	}
	if controllers > 1 && !p.warnedLeaderAddress {
		p.log.Warnf("Joining nodes use the address of the leading controller, which changes with the leader. " +
			"Set spec.api.externalAddress or a control plane load balancing virtual IP.")
	}
	p.warnedLeaderAddress = controllers > 1
}

func (p *Publisher) input(cfg *v1beta1.ClusterConfig, endpoint string) (*Input, error) {
	ca, err := os.ReadFile(filepath.Join(p.K0sVars.CertRootDir, "ca.crt"))
	if err != nil {
		return nil, err
	}
	network := p.NodeConfig.Spec.Network
	family := p.NodeConfig.Spec.PrimaryAddressFamily()
	dnsAddress, err := network.DNSAddress(family)
	if err != nil {
		return nil, err
	}
	return &Input{
		ControlPlaneEndpoint: endpoint,
		CACertPEM:            ca,
		KubernetesVersion:    kubernetesVersion(),
		ServiceCIDR:          network.BuildServiceCIDR(family),
		PodCIDR:              cfg.Spec.Network.BuildPodCIDR(family),
		ClusterDomain:        network.ClusterDomain,
		DNSAddress:           dnsAddress,
	}, nil
}

func kubernetesVersion() string {
	if v := build.KubernetesVersion; v != "" {
		return "v" + strings.TrimPrefix(v, "v")
	}
	return constant.KubeProxyImageVersion
}
