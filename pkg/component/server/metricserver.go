/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"context"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/util"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

const metricServerTemplate = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system-aggregated-metrics-reader
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch"]
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
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
  name: metrics-server-system-auth-delegator
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
kind: ClusterRole
metadata:
  name: system-metrics-server
rules:
  - apiGroups:
      - ""
    resources:
      - pods
      - nodes
      - nodes/stats
      - namespaces
      - configmaps
    verbs:
      - get
      - list
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system-metrics-server
subjects:
  - kind: ServiceAccount
    name: metrics-server
    namespace: kube-system
---
apiVersion: apiregistration.k8s.io/v1beta1
kind: APIService
metadata:
  name: v1beta1.metrics.k8s.io
spec:
  service:
    name: metrics-server
    namespace: kube-system
  group: metrics.k8s.io
  version: v1beta1
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 100
  versionPriority: 100
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    k8s-app: metrics-server
spec:
  selector:
    matchLabels:
      k8s-app: metrics-server
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      name: metrics-server
      labels:
        k8s-app: metrics-server
    spec:
      serviceAccountName: metrics-server
      volumes:
      # mount in tmp so we can safely use from-scratch images and/or read-only containers
      - name: tmp-dir
        emptyDir: {}
      priorityClassName: system-cluster-critical
      containers:
      - name: metrics-server
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        args:
          - --cert-dir=/tmp
          - --secure-port=4443
          - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
         # Until we have proper serving cert (signed by cluster CA & proper IP sans etc.) on kubelet, not much else we can do
          - --kubelet-insecure-tls
        ports:
        - name: https
          containerPort: 4443
          protocol: TCP
        resources:
          requests:
            memory: {{ .MEMRequest }}
            cpu: {{ .CPURequest }}
        readinessProbe:
          httpGet:
            path: /healthz
            port: https
            scheme: HTTPS
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
        volumeMounts:
        - name: tmp-dir
          mountPath: /tmp
      nodeSelector:
        kubernetes.io/os: linux
---
apiVersion: v1
kind: Service
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    kubernetes.io/name: "Metrics-server"
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    k8s-app: metrics-server
  ports:
  - port: 443
    protocol: TCP
    targetPort: https
---
`

// MetricServer is the reconciler implementation for metrics server
type MetricServer struct {
	log               *logrus.Entry
	clusterConfig     *config.ClusterConfig
	tickerDone        chan struct{}
	K0sVars           constant.CfgVars
	kubeClientFactory k8sutil.ClientFactory
}

type metricsConfig struct {
	Image      string
	CPURequest string
	MEMRequest string
}

// NewMetricServer creates new MetricServer reconciler
func NewMetricServer(clusterConfig *config.ClusterConfig, k0sVars constant.CfgVars, kubeClientFactory k8sutil.ClientFactory) (*MetricServer, error) {
	log := logrus.WithFields(logrus.Fields{"component": "metricServer"})
	return &MetricServer{
		log:               log,
		clusterConfig:     clusterConfig,
		K0sVars:           k0sVars,
		kubeClientFactory: kubeClientFactory,
	}, nil
}

// Init does nothing
func (m *MetricServer) Init() error {
	return nil
}

// Run runs the metric server reconciler
func (m *MetricServer) Run() error {
	m.tickerDone = make(chan struct{})

	msDir := path.Join(m.K0sVars.ManifestsDir, "metricserver")
	err := util.InitDirectory(msDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		var previousConfig = metricsConfig{}
		for {
			select {
			case <-ticker.C:
				newConfig, err := m.getConfig()
				if err != nil {
					m.log.Warnf("failed to calculate metrics-server config: %s", err.Error())
				}
				if previousConfig == newConfig {
					continue
				}
				tw := util.TemplateWriter{
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
			case <-m.tickerDone:
				m.log.Info("metric server reconciler done")
				return
			}
		}
	}()

	return nil
}

// Stop stops the reconciler
func (m *MetricServer) Stop() error {
	close(m.tickerDone)
	return nil
}

// Healthy is the health-check interface
func (m *MetricServer) Healthy() error { return nil }

// Mostly for calculating the resource needs based on node numbers. From https://github.com/kubernetes-sigs/metrics-server#scaling :
// Starting from v0.5.0 Metrics Server comes with default resource requests that should guarantee good performance for most cluster configurations up to 100 nodes:
// - 100m core of CPU
// - 300MiB of memory
// So that's 10m CPU and 30MiB mem per 10 nodes
func (m *MetricServer) getConfig() (metricsConfig, error) {
	cfg := metricsConfig{
		Image: m.clusterConfig.Images.MetricsServer.URI(),
	}

	kubeClient, err := m.kubeClientFactory.GetClient()
	if err != nil {
		return cfg, err
	}

	nodeList, err := kubeClient.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
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
