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
	"math"
	"path"
	"path/filepath"
	"reflect"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

const coreDNSTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system-coredns
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - services
  - pods
  - namespaces
  verbs:
  - list
  - watch
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system-coredns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system-coredns
subjects:
- kind: ServiceAccount
  name: coredns
  namespace: kube-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health
        ready
        kubernetes {{ .ClusterDomain }} in-addr.arpa ip6.arpa {
          pods insecure
          ttl 30
          fallthrough in-addr.arpa ip6.arpa
        }
        prometheus :9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/name: "CoreDNS"
spec:
  replicas: {{ .Replicas }}
  strategy:
    type: RollingUpdate
    {{- if .MaxUnavailableReplicas }}
    rollingUpdate:
      maxUnavailable: {{ .MaxUnavailableReplicas }}
    {{- end }}
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
      annotations:
        prometheus.io/scrape: 'true'
        prometheus.io/port: '9153'
    spec:
      serviceAccountName: coredns
      tolerations:
        - key: "CriticalAddonsOnly"
          operator: "Exists"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      nodeSelector:
        kubernetes.io/os: linux
      {{- if not .DisablePodAntiAffinity }}
      # Require running coredns replicas on different nodes
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - topologyKey: "kubernetes.io/hostname"
            labelSelector:
              matchExpressions:
              - key: k8s-app
                operator: In
                values: ['kube-dns']
      {{- end }}
      containers:
      - name: coredns
        image: {{ .Image }}
        imagePullPolicy: {{ .PullPolicy }}
        resources:
          limits:
            memory: 170Mi
          requests:
            cpu: 100m
            memory: 70Mi
        args: [ "-conf", "/etc/coredns/Corefile" ]
        volumeMounts:
        - name: config-volume
          mountPath: /etc/coredns
          readOnly: true
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        - containerPort: 9153
          name: metrics
          protocol: TCP
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            add:
            - NET_BIND_SERVICE
            drop:
            - all
          readOnlyRootFilesystem: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          periodSeconds: 10
          timeoutSeconds: 1
          successThreshold: 1
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 8181
            scheme: HTTP
          initialDelaySeconds: 30 # give loop plugin time to detect loops
          periodSeconds: 2
          timeoutSeconds: 1
          successThreshold: 1
          failureThreshold: 3
      dnsPolicy: Default
      volumes:
        - name: config-volume
          configMap:
            name: coredns
            items:
            - key: Corefile
              path: Corefile
{{- if not .DisablePodDisruptionBudget }}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: coredns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/name: "CoreDNS"
spec:
  minAvailable: 50%
  selector:
    matchLabels:
      k8s-app: kube-dns
{{- end }}
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  annotations:
    prometheus.io/port: "9153"
    prometheus.io/scrape: "true"
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "CoreDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: {{ .ClusterDNSIP }}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
  - name: metrics
    port: 9153
    protocol: TCP
`

const HostsPerExtraReplica = 10.0

var _ manager.Component = (*CoreDNS)(nil)
var _ manager.Reconciler = (*CoreDNS)(nil)

// CoreDNS is the component implementation to manage CoreDNS
type CoreDNS struct {
	K0sVars    *config.CfgVars
	NodeConfig *v1beta1.ClusterConfig

	client                 kubernetes.Interface
	log                    *logrus.Entry
	manifestDir            string
	previousConfig         coreDNSConfig
	stopFunc               context.CancelFunc
	lastKnownClusterConfig *v1beta1.ClusterConfig
}

type coreDNSConfig struct {
	Replicas                   int
	ClusterDNSIP               string
	ClusterDomain              string
	Image                      string
	PullPolicy                 string
	MaxUnavailableReplicas     *uint
	DisablePodAntiAffinity     bool
	DisablePodDisruptionBudget bool
}

// NewCoreDNS creates new instance of CoreDNS component
func NewCoreDNS(k0sVars *config.CfgVars, clientFactory k8sutil.ClientFactoryInterface, nodeConfig *v1beta1.ClusterConfig) (*CoreDNS, error) {
	manifestDir := path.Join(k0sVars.ManifestsDir, "coredns")

	client, err := clientFactory.GetClient()
	if err != nil {
		return nil, err
	}
	log := logrus.WithFields(logrus.Fields{"component": "coredns"})
	return &CoreDNS{
		client:      client,
		log:         log,
		K0sVars:     k0sVars,
		manifestDir: manifestDir,
		NodeConfig:  nodeConfig,
	}, nil
}

// Init does nothing
func (c *CoreDNS) Init(_ context.Context) error {
	return dir.Init(c.manifestDir, constant.ManifestsDirMode)
}

// Run runs the CoreDNS reconciler component
func (c *CoreDNS) Start(ctx context.Context) error {
	ctx, c.stopFunc = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if c.lastKnownClusterConfig == nil {
					// We cannot figure out the full config without having the last known cluster config from CR
					continue
				}
				err := c.Reconcile(ctx, c.lastKnownClusterConfig)
				if err != nil {
					c.log.Warnf("failed to reconcile coredns based on node count: %v", err)
				}
			case <-ctx.Done():
				c.log.Info("coredns node reconciler done")
				return
			}
		}
	}()

	return nil
}

func (c *CoreDNS) getConfig(ctx context.Context, clusterConfig *v1beta1.ClusterConfig) (coreDNSConfig, error) {
	nodeConfig, err := c.K0sVars.NodeConfig()
	if err != nil {
		return coreDNSConfig{}, fmt.Errorf("failed to get node config: %w", err)
	}

	dns, err := nodeConfig.Spec.Network.DNSAddress()
	if err != nil {
		return coreDNSConfig{}, err
	}

	nodes, err := c.client.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return coreDNSConfig{}, err
	}

	nodeCount := len(nodes.Items)

	config := coreDNSConfig{
		Replicas:      replicaCount(nodeCount),
		ClusterDomain: nodeConfig.Spec.Network.ClusterDomain,
		ClusterDNSIP:  dns,
		Image:         clusterConfig.Spec.Images.CoreDNS.URI(),
		PullPolicy:    clusterConfig.Spec.Images.DefaultPullPolicy,
	}

	if config.Replicas <= 1 {

		// No pod anti-affinity for single replica deployments, so that CoreDNS
		// can be rolled on a single node without downtime, as the single
		// replica can remain operational until the new replica can take over.
		config.DisablePodAntiAffinity = true

		// No PodDisruptionBudget for single replica deployments, so that
		// CoreDNS may be drained from the node running the single replica. This
		// might leave CoreDNS eligible for eviction due to node pressure, but
		// such clusters aren't HA in the first place and it seems more
		// desirable to not block a drain.
		config.DisablePodDisruptionBudget = true

	} else if config.Replicas == 2 || config.Replicas == 3 {

		// Set maxUnavailable=1 only for deployments with two or three replicas.
		// Use the Kubernetes defaults (maxUnavailable=25%, rounded down to get
		// absolute values) for all other cases. For single replica deployments,
		// maxUnavailable=1 would mean that the deployment's available condition
		// would be true even with zero ready replicas. For deployments with 4
		// to 7 replicas, it would be the same as the default, and for
		// deployments with 8 or more replicas, this would artificially
		// constrain the rolling update speed.
		config.MaxUnavailableReplicas = ptr.To(uint(1))

	}

	return config, nil
}

// calculates an extra replica per 10 hosts
func replicaCount(nodeCount int) int {
	// always at least one so we get the coreDNS up-and running fast with the first node joining the cluster
	if nodeCount <= 1 {
		return 1
	}
	extraReplicas := int(math.Ceil(float64(nodeCount) / HostsPerExtraReplica))
	return 1 + extraReplicas
}

// Stop stops the CoreDNS reconciler
func (c *CoreDNS) Stop() error {
	if c.stopFunc != nil {
		logrus.Debug("closing coreDNS component context")
		c.stopFunc()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (c *CoreDNS) Reconcile(ctx context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	logrus.Debug("reconcile method called for: CoreDNS")
	cfg, err := c.getConfig(ctx, clusterConfig)
	if err != nil {
		return fmt.Errorf("error calculating coredns configs: %w, will retry", err)
	}
	if reflect.DeepEqual(c.previousConfig, cfg) {
		c.log.Infof("current cfg matches existing, not gonna do anything")
		return nil
	}
	tw := templatewriter.TemplateWriter{
		Name:     "coredns",
		Template: coreDNSTemplate,
		Data:     cfg,
		Path:     filepath.Join(c.manifestDir, "coredns.yaml"),
	}
	err = tw.Write()
	if err != nil {
		return fmt.Errorf("error writing coredns manifests: %w, will retry", err)
	}
	c.previousConfig = cfg
	c.lastKnownClusterConfig = clusterConfig
	return nil
}
