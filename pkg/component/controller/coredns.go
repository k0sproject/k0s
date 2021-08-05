/*
Copyright 2021 k0s authors

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
	"math"
	"path"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
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
  - ""
  resources:
  - nodes
  verbs:
  - get
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
  replicas: {{ .Replicas}}
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
    spec:
      serviceAccountName: coredns
      tolerations:
        - key: "CriticalAddonsOnly"
          operator: "Exists"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      nodeSelector:
        beta.kubernetes.io/os: linux
      # Prefer running coredns replicas on different nodes
      affinity:
        preferredDuringSchedulingIgnoredDuringExecution:
          topologyKey: "kubernetes.io/hostname"
          labelSelector:
            matchExpressions:
            - key: k8s-app
              operator: In
              values: ['kube-dns']
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
          initialDelaySeconds: 0
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

// CoreDNS is the component implementation to manage CoreDNS
type CoreDNS struct {
	client        kubernetes.Interface
	tickerDone    chan struct{}
	log           *logrus.Entry
	clusterConfig *v1beta1.ClusterConfig
	K0sVars       constant.CfgVars
}

type coreDNSConfig struct {
	Replicas      int
	ClusterDNSIP  string
	ClusterDomain string
	Image         string
	PullPolicy    string
}

// NewCoreDNS creates new instance of CoreDNS component
func NewCoreDNS(clusterConfig *v1beta1.ClusterConfig, k0sVars constant.CfgVars, clientFactory k8sutil.ClientFactoryInterface) (*CoreDNS, error) {
	client, err := clientFactory.GetClient()
	if err != nil {
		return nil, err
	}
	log := logrus.WithFields(logrus.Fields{"component": "coredns"})
	return &CoreDNS{
		client:        client,
		log:           log,
		clusterConfig: clusterConfig,
		K0sVars:       k0sVars,
	}, nil
}

// Init does nothing
func (c *CoreDNS) Init() error {
	return nil
}

// Run runs the CoreDNS reconciler component
func (c *CoreDNS) Run() error {
	corednsDir := path.Join(c.K0sVars.ManifestsDir, "coredns")
	err := dir.Init(corednsDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	c.tickerDone = make(chan struct{})

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		previousConfig := coreDNSConfig{}
		for {
			select {
			case <-ticker.C:
				cfg, err := c.getConfig()
				if err != nil {
					c.log.Errorf("error calculating coredns configs: %s. will retry", err.Error())
					continue
				}
				if cfg == previousConfig {
					c.log.Infof("current cfg matches existing, not gonna do anything")
					continue
				}
				tw := templatewriter.TemplateWriter{
					Name:     "coredns",
					Template: coreDNSTemplate,
					Data:     cfg,
					Path:     filepath.Join(corednsDir, "coredns.yaml"),
				}
				err = tw.Write()
				if err != nil {
					c.log.Errorf("error writing coredns manifests: %s. will retry", err.Error())
					continue
				}
				previousConfig = cfg
			case <-c.tickerDone:
				c.log.Info("coredns reconciler done")
				return
			}
		}
	}()

	return nil
}

func (c *CoreDNS) getConfig() (coreDNSConfig, error) {
	dns, err := c.clusterConfig.Spec.Network.DNSAddress()
	if err != nil {
		return coreDNSConfig{}, err
	}

	nodes, err := c.client.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return coreDNSConfig{}, err
	}

	nodeCount := len(nodes.Items)
	replicas := replicaCount(nodeCount)

	config := coreDNSConfig{
		Replicas:      replicas,
		ClusterDomain: "cluster.local",
		ClusterDNSIP:  dns,
		Image:         c.clusterConfig.Spec.Images.CoreDNS.URI(),
		PullPolicy:    c.clusterConfig.Spec.Images.DefaultPullPolicy,
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
	if c.tickerDone != nil {
		close(c.tickerDone)
	}
	return nil
}

// Health-check interface
func (c *CoreDNS) Healthy() error { return nil }
