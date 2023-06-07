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
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

const metricServerTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    k8s-app: metrics-server
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-view: "true"
  name: system:aggregated-metrics-reader
rules:
- apiGroups:
  - metrics.k8s.io
  resources:
  - pods
  - nodes
  verbs:
  - get
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    k8s-app: metrics-server
  name: system:metrics-server
rules:
- apiGroups:
  - ""
  resources:
  - nodes/metrics
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  verbs:
  - get
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server-auth-reader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    k8s-app: metrics-server
  name: system:metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: v1
kind: Service
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server
  namespace: kube-system
spec:
  ports:
  - name: https
    port: 443
    protocol: TCP
    targetPort: https
  selector:
    k8s-app: metrics-server
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: metrics-server
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      labels:
        k8s-app: metrics-server
    spec:
      containers:
      - args:
        - --cert-dir=/tmp
        - --secure-port=10250
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --kubelet-use-node-status-port
        - --metric-resolution=15s
        image: {{ .Image }}
        imagePullPolicy: {{ .PullPolicy }}
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /livez
            port: https
            scheme: HTTPS
          periodSeconds: 10
        name: metrics-server
        ports:
        - containerPort: 10250
          name: https
          protocol: TCP
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /readyz
            port: https
            scheme: HTTPS
          initialDelaySeconds: 20
          periodSeconds: 10
        resources:
          requests:
            memory: {{ .MEMRequest }}
            cpu: {{ .CPURequest }}
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
        volumeMounts:
        - mountPath: /tmp
          name: tmp-dir
      nodeSelector:
        kubernetes.io/os: linux
      tolerations:
      - key: "node-role.kubernetes.io/master"
        operator: "Exists"
        effect: "NoSchedule"
      priorityClassName: system-cluster-critical
      serviceAccountName: metrics-server
      volumes:
      - emptyDir: {}
        name: tmp-dir
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  labels:
    k8s-app: metrics-server
  name: v1beta1.metrics.k8s.io
spec:
  group: metrics.k8s.io
  groupPriorityMinimum: 100
  insecureSkipTLSVerify: true
  service:
    name: metrics-server
    namespace: kube-system
  version: v1beta1
  versionPriority: 100
`

// MetricServer is the reconciler implementation for metrics server
type MetricServer struct {
	log logrus.FieldLogger

	K0sVars           *config.CfgVars
	kubeClientFactory k8sutil.ClientFactoryInterface

	clusterConfig *v1beta1.ClusterConfig
	tickerDone    context.CancelFunc
}

type metricsConfig struct {
	Image      string
	PullPolicy string
	CPURequest string
	MEMRequest string
}

var _ manager.Component = (*MetricServer)(nil)
var _ manager.Reconciler = (*MetricServer)(nil)

// NewMetricServer creates new MetricServer reconciler
func NewMetricServer(k0sVars *config.CfgVars, kubeClientFactory k8sutil.ClientFactoryInterface) *MetricServer {
	return &MetricServer{
		log: logrus.WithFields(logrus.Fields{"component": "metricServer"}),

		K0sVars:           k0sVars,
		kubeClientFactory: kubeClientFactory,
	}
}

// Init does nothing
func (m *MetricServer) Init(_ context.Context) error {
	return nil
}

// Run runs the metric server reconciler
func (m *MetricServer) Start(ctx context.Context) error {
	ctx, m.tickerDone = context.WithCancel(ctx)

	msDir := path.Join(m.K0sVars.ManifestsDir, "metricserver")
	err := dir.Init(msDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		previousConfig := metricsConfig{}
		for {
			select {
			case <-ticker.C:
				newConfig, err := m.getConfig(ctx)
				if err != nil {
					m.log.Warnf("failed to calculate metrics-server config: %s", err.Error())
					continue
				}
				if previousConfig == newConfig {
					continue
				}
				tw := templatewriter.TemplateWriter{
					Name:     "metricServer",
					Template: metricServerTemplate,
					Data:     newConfig,
					Path:     filepath.Join(msDir, "metric_server.yaml"),
				}
				err = tw.Write()
				if err != nil {
					m.log.Errorf("error writing metric server manifests: %s. will retry", err.Error())
					continue
				}
				previousConfig = newConfig
			case <-ctx.Done():
				m.log.Info("metric server reconciler done")
				return
			}
		}
	}()

	return nil
}

// Stop stops the reconciler
func (m *MetricServer) Stop() error {
	if m.tickerDone != nil {
		m.tickerDone()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (m *MetricServer) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	logrus.Debug("reconcile method called for: MetricServer")
	// We just store the last known config, the main reconciler ticker will reconcile config based on number of nodes etc.
	m.clusterConfig = clusterConfig
	return nil
}

// Mostly for calculating the resource needs based on node numbers. From https://github.com/kubernetes-sigs/metrics-server#scaling :
// Starting from v0.5.0 Metrics Server comes with default resource requests that should guarantee good performance for most cluster configurations up to 100 nodes:
// - 100m core of CPU
// - 300MiB of memory
// So that's 10m CPU and 30MiB mem per 10 nodes
func (m *MetricServer) getConfig(ctx context.Context) (metricsConfig, error) {
	if m.clusterConfig == nil {
		return metricsConfig{}, fmt.Errorf("cluster config not available yet")
	}
	cfg := metricsConfig{
		Image:      m.clusterConfig.Spec.Images.MetricsServer.URI(),
		PullPolicy: m.clusterConfig.Spec.Images.DefaultPullPolicy,
	}

	kubeClient, err := m.kubeClientFactory.GetClient()
	if err != nil {
		return cfg, err
	}

	nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return cfg, err
	}

	scale := math.Ceil(float64(len(nodeList.Items)) / 10.0)
	if scale < 1 {
		scale = 1
	}
	memRequest := int(30 * scale)
	cpuRequest := int(10 * scale)

	cfg.MEMRequest = fmt.Sprintf("%dM", memRequest)
	cfg.CPURequest = fmt.Sprintf("%dm", cpuRequest)

	return cfg, nil
}
