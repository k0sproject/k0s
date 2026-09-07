// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/metadata"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
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
      priorityClassName: system-cluster-critical
      serviceAccountName: coredns
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
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
      topologySpreadConstraints:
      - topologyKey: topology.kubernetes.io/zone
        maxSkew: 1
        whenUnsatisfiable: ScheduleAnyway
        labelSelector:
          matchLabels:
            k8s-app: kube-dns
      containers:
      - name: coredns
        image: {{ .Image }}
        imagePullPolicy: {{ .PullPolicy }}
        resources:
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
          runAsNonRoot: true
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

// CoreDNSStackName is the name of the in-memory applier stack managing the CoreDNS resources.
const CoreDNSStackName = "coredns"

const HostsPerExtraReplica = 10.0

var _ manager.Component = (*CoreDNS)(nil)
var _ manager.Reconciler = (*CoreDNS)(nil)

// CoreDNS is the component implementation to manage CoreDNS
type CoreDNS struct {
	dnsAddress             string
	clusterDomain          string
	client                 metadata.Interface
	clientFactory          k8sutil.ClientFactoryInterface
	leaderElector          leaderelector.Interface
	log                    *logrus.Entry
	previousConfig         coreDNSConfig
	previousPatches        v1beta1.Patches
	stopFunc               context.CancelFunc
	lastKnownClusterConfig value.Latest[*v1beta1.ClusterConfig]
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
func NewCoreDNS(clientFactory k8sutil.ClientFactoryInterface, leaderElector leaderelector.Interface, nodeConfig *v1beta1.ClusterConfig) (*CoreDNS, error) {
	dnsAddress, err := nodeConfig.Spec.Network.DNSAddress(nodeConfig.Spec.PrimaryAddressFamily())
	if err != nil {
		return nil, err
	}

	restConfig, err := clientFactory.GetRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := metadata.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &CoreDNS{
		dnsAddress:    dnsAddress,
		clusterDomain: nodeConfig.Spec.Network.ClusterDomain,
		client:        client,
		clientFactory: clientFactory,
		leaderElector: leaderElector,
		log:           logrus.WithField("component", "coredns"),
	}, nil
}

// Init does nothing as there's nothing to initialize
func (c *CoreDNS) Init(_ context.Context) error {
	return nil
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
				clusterConfig, _ := c.lastKnownClusterConfig.Peek()
				if clusterConfig == nil {
					// We cannot figure out the full config without having the last known cluster config from CR
					continue
				}
				err := c.Reconcile(ctx, clusterConfig)
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
	nodes, err := c.client.Resource(corev1.SchemeGroupVersion.WithResource("nodes")).List(ctx, metav1.ListOptions{
		LabelSelector: fields.OneTermEqualSelector(corev1.LabelOSStable, string(corev1.Linux)).String(),
	})
	if err != nil {
		return coreDNSConfig{}, err
	}

	nodeCount := len(nodes.Items)

	config := coreDNSConfig{
		Replicas:      replicaCount(nodeCount),
		ClusterDomain: c.clusterDomain,
		ClusterDNSIP:  c.dnsAddress,
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
		config.MaxUnavailableReplicas = new(uint(1))

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

	// Needed regardless of leadership, so the node-count ticker in Start can
	// retry once this instance becomes the leader.
	c.lastKnownClusterConfig.Set(clusterConfig)

	if status, _ := c.leaderElector.CurrentStatus(); status != leaderelection.StatusLeading {
		return nil
	}

	cfg, err := c.getConfig(ctx, clusterConfig)
	if err != nil {
		return fmt.Errorf("error calculating coredns configs: %w, will retry", err)
	}
	var patches v1beta1.Patches
	if cd := clusterConfig.Spec.Network.CoreDNS; cd != nil {
		patches = cd.Patches
	}
	if reflect.DeepEqual(c.previousConfig, cfg) &&
		reflect.DeepEqual(c.previousPatches, patches) {
		c.log.Debug("Configuration is up to date, not gonna do anything")
		return nil
	}
	tw := templatewriter.TemplateWriter{
		Name:     "coredns",
		Template: coreDNSTemplate,
		Data:     cfg,
		Patches:  patches,
	}
	var out bytes.Buffer
	err = tw.WriteToBuffer(&out)
	if err != nil {
		return fmt.Errorf("error writing coredns manifests: %w, will retry", err)
	}
	resources, err := applier.ReadUnstructuredStream(&out, CoreDNSStackName)
	if err != nil {
		return fmt.Errorf("error reading coredns resources: %w, will retry", err)
	}
	if err := applier.ApplyStack(ctx, c.clientFactory, resources, CoreDNSStackName); err != nil {
		return fmt.Errorf("error applying coredns stack: %w, will retry", err)
	}

	c.previousConfig = cfg
	c.previousPatches = patches
	return nil
}
