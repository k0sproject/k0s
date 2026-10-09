// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package externalnodes runs copies of the node-local load balanced DaemonSets on nodes without k0s.
// Such nodes are labeled by k0s, and the copies use the control plane endpoint directly.
package externalnodes

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sirupsen/logrus"
)

const (
	// The stack in the manifests directory.
	stackName = "external-nodes"
	// The label selector of nodes without k0s.
	externalNodeSelector = constant.K0sNodeLabel + "=false"
)

// Component keeps the copies of the node-local load balanced DaemonSets while nodes without k0s exist.
type Component struct {
	K0sVars *config.CfgVars
	// The node-local configuration, for the control plane endpoint.
	NodeConfig *v1beta1.ClusterConfig
	Clients    kubeutil.ClientFactoryInterface

	log         logrus.FieldLogger
	config      value.Latest[*v1beta1.ClusterConfig]
	stop        func()
	manifestDir string
	// How often the copies are refreshed, besides on changes of nodes without k0s.
	resync time.Duration
}

var (
	_ manager.Component  = (*Component)(nil)
	_ manager.Reconciler = (*Component)(nil)
)

func (c *Component) Init(context.Context) error {
	c.log = logrus.WithField("component", stackName)
	// The stack only exists while needed, so clusters without such nodes see no changes.
	c.manifestDir = filepath.Join(c.K0sVars.ManifestsDir, stackName)
	if c.resync == 0 {
		c.resync = 30 * time.Second
	}
	return nil
}

// Reconcile implements [manager.Reconciler].
func (c *Component) Reconcile(_ context.Context, cfg *v1beta1.ClusterConfig) error {
	c.config.Set(cfg.DeepCopy())
	return nil
}

func (c *Component) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	nodesChanged := make(chan struct{}, 1)
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		c.watchNodes(ctx, nodesChanged)
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(c.resync)
		defer ticker.Stop()
		cfg, changed := c.config.Peek()
		for {
			if cfg != nil {
				if err := c.sync(ctx, cfg); err != nil {
					c.log.WithError(err).Error("Failed to update the manifests, retrying")
				}
			}
			select {
			case <-changed:
				cfg, changed = c.config.Peek()
			case <-nodesChanged:
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
	c.stop = func() { cancel(); <-done; <-watched }
	return nil
}

func (c *Component) Stop() error {
	if c.stop != nil {
		c.stop()
	}
	return nil
}

// watchNodes signals when nodes without k0s come or go, so that their copies follow right away.
func (c *Component) watchNodes(ctx context.Context, changed chan<- struct{}) {
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		client, err := c.Clients.GetClient()
		if err != nil {
			c.log.WithError(err).Error("Failed to create a client, retrying")
			return
		}

		factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = externalNodeSelector
		}))
		defer factory.Shutdown()
		notify := func(any) {
			select {
			case changed <- struct{}{}:
			default:
			}
		}
		_, err = factory.Core().V1().Nodes().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    notify,
			DeleteFunc: notify,
		})
		if err != nil {
			c.log.WithError(err).Error("Failed to watch nodes, retrying")
			return
		}

		factory.Start(ctx.Done())
		<-ctx.Done()
	}, 10*time.Second)
}

func (c *Component) sync(ctx context.Context, cfg *v1beta1.ClusterConfig) error {
	if cfg.Spec.Network == nil || !cfg.Spec.Network.NodeLocalLoadBalancing.IsEnabled() {
		return os.RemoveAll(c.manifestDir)
	}
	exist, err := c.externalNodesExist(ctx)
	if err != nil {
		return err
	}
	if !exist {
		return os.RemoveAll(c.manifestDir)
	}

	endpoint, err := ControlPlaneEndpoint(c.NodeConfig.Spec)
	if err != nil {
		return err
	}
	copies, err := c.copies(ctx, cfg, endpoint)
	if err != nil {
		return err
	}
	if len(copies) == 0 {
		return os.RemoveAll(c.manifestDir)
	}
	err = dir.Init(c.manifestDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	return applier.WriteManifest(filepath.Join(c.manifestDir, "daemonsets.yaml"), copies)
}

// externalNodesExist reports whether nodes without k0s exist.
func (c *Component) externalNodesExist(ctx context.Context) (bool, error) {
	client, err := c.Clients.GetClient()
	if err != nil {
		return false, err
	}
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: externalNodeSelector, Limit: 1})
	if err != nil {
		return false, err
	}
	return len(nodes.Items) > 0, nil
}

// copies derives copies of the DaemonSets that point at node-local load balancers.
// Only those that actually use localhost are copied.
func (c *Component) copies(ctx context.Context, cfg *v1beta1.ClusterConfig, endpoint string) ([]runtime.Object, error) {
	client, err := c.Clients.GetClient()
	if err != nil {
		return nil, err
	}
	daemonSets := client.AppsV1().DaemonSets(metav1.NamespaceSystem)

	var copies []runtime.Object
	var cm *corev1.ConfigMap
	cm, err = client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, kubeProxyName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	var ds *appsv1.DaemonSet
	if cm != nil && err == nil && usesLocalhost(cm) {
		ds, err = daemonSets.Get(ctx, kubeProxyName, metav1.GetOptions{})
		switch {
		case err == nil:
			var cmCopy *corev1.ConfigMap
			var dsCopy *appsv1.DaemonSet
			cmCopy, dsCopy, err = kubeProxyCopy(cm, ds, "https://"+endpoint)
			if err != nil {
				return nil, err
			}
			copies = append(copies, cmCopy, dsCopy)
		case !apierrors.IsNotFound(err):
			return nil, err
		}
	}

	ds, err = daemonSets.Get(ctx, konnectivityName, metav1.GetOptions{})
	switch {
	case err == nil && hasArg(ds, "--proxy-server-host=localhost"):
		// The agents only reach every konnectivity server if the endpoint balances the agent port, too,
		// as a CPLB virtual IP does with loadBalancedPorts.konnectivity.
		var host string
		host, _, err = net.SplitHostPort(endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid control plane endpoint %q: %w", endpoint, err)
		}
		port := v1beta1.DefaultKonnectivitySpec().AgentPort
		if p := cfg.Spec.Konnectivity; p != nil && p.AgentPort > 0 {
			port = p.AgentPort
		}
		copies = append(copies, konnectivityCopy(ds, host, port))
	case err != nil && !apierrors.IsNotFound(err):
		return nil, err
	}
	return copies, nil
}

// usesLocalhost reports whether the kube-proxy kubeconfig points at a node-local load balancer.
func usesLocalhost(cm *corev1.ConfigMap) bool {
	kubeconfig, err := clientcmd.Load([]byte(cm.Data[kubeProxyConfKey]))
	if err != nil {
		return false
	}
	var u *url.URL
	for _, cluster := range kubeconfig.Clusters {
		u, err = url.Parse(cluster.Server)
		if err == nil && u.Hostname() == "localhost" {
			return true
		}
	}
	return false
}

func hasArg(ds *appsv1.DaemonSet, arg string) bool {
	containers := ds.Spec.Template.Spec.Containers
	for i := range containers {
		if slices.Contains(containers[i].Args, arg) {
			return true
		}
	}
	return false
}
