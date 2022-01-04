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
	"os"
	"path"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

// KubeProxy is the component implementation to manage kube-proxy
type KubeProxy struct {
	log            *logrus.Entry
	nodeConf       *v1beta1.ClusterConfig
	K0sVars        constant.CfgVars
	previousConfig proxyConfig
	manifestDir    string
}

var _ component.Component = &KubeProxy{}
var _ component.ReconcilerComponent = &KubeProxy{}

// NewKubeProxy creates new KubeProxy component
func NewKubeProxy(configFile string, k0sVars constant.CfgVars) (*KubeProxy, error) {
	log := logrus.WithFields(logrus.Fields{"component": "kubeproxy"})
	cfg, err := config.GetNodeConfig(configFile, k0sVars)
	if err != nil {
		return nil, err
	}
	proxyDir := path.Join(k0sVars.ManifestsDir, "kubeproxy")
	return &KubeProxy{
		log:            log,
		nodeConf:       cfg,
		K0sVars:        k0sVars,
		previousConfig: proxyConfig{},
		manifestDir:    proxyDir,
	}, nil
}

// Init does nothing
func (k *KubeProxy) Init(_ context.Context) error {
	return nil
}

// Run runs the kube-proxy reconciler
func (k *KubeProxy) Run(_ context.Context) error { return nil }

// Reconcile detects changes in configuration and applies them to the component
func (k *KubeProxy) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	if clusterConfig.Spec.Network.KubeProxy.Disabled {
		return os.RemoveAll(k.manifestDir)
	}
	err := dir.Init(k.manifestDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	cfg, err := k.getConfig(clusterConfig)
	if err != nil {
		return err
	}
	if cfg == k.previousConfig {
		k.log.Infof("current cfg matches existing, not gonna do anything")
		return nil
	}
	tw := templatewriter.TemplateWriter{
		Name:     "kube-proxy",
		Template: proxyTemplate,
		Data:     cfg,
		Path:     filepath.Join(k.manifestDir, "kube-proxy.yaml"),
	}
	err = tw.Write()
	if err != nil {
		k.log.Errorf("error writing kube-proxy manifests: %s. will retry", err.Error())
	}
	k.previousConfig = cfg

	return nil
}

// Stop stop the reconcilier
func (k *KubeProxy) Stop() error {
	return nil
}

func (k *KubeProxy) getConfig(clusterConfig *v1beta1.ClusterConfig) (proxyConfig, error) {
	cfg := proxyConfig{
		ClusterCIDR:          clusterConfig.Spec.Network.BuildPodCIDR(),
		ControlPlaneEndpoint: clusterConfig.Spec.API.APIAddressURL(),
		Image:                clusterConfig.Spec.Images.KubeProxy.URI(),
		PullPolicy:           clusterConfig.Spec.Images.DefaultPullPolicy,
		DualStack:            clusterConfig.Spec.Network.DualStack.Enabled,
		Mode:                 clusterConfig.Spec.Network.KubeProxy.Mode,
	}

	return cfg, nil
}

type proxyConfig struct {
	DualStack            bool
	ControlPlaneEndpoint string
	ClusterCIDR          string
	Image                string
	PullPolicy           string
	Mode                 string
}

const proxyTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-proxy
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: kube-proxy
  namespace: kube-system
rules:
  - apiGroups: [""]
    verbs: ["get"]
    resources: ["configmaps"]
    resourceNames: ["kube-proxy"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: node-proxier
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-proxier
subjects:
- kind: ServiceAccount
  name: kube-proxy
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: kube-proxy-conf
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-proxy
subjects:
- kind: Group
  name: system:bootstrappers
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: kube-proxy
  namespace: kube-system
  labels:
    app: kube-proxy
data:
  kubeconfig.conf: |-
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        server: {{ .ControlPlaneEndpoint }}
      name: default
    contexts:
    - context:
        cluster: default
        namespace: default
        user: default
      name: default
    current-context: default
    users:
    - name: default
      user:
        tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
  config.conf: |-
    apiVersion: kubeproxy.config.k8s.io/v1alpha1
    bindAddress: 0.0.0.0
    clientConnection:
      acceptContentTypes: ""
      burst: 0
      contentType: ""
      kubeconfig: /var/lib/kube-proxy/kubeconfig.conf
      qps: 0
    clusterCIDR: {{ .ClusterCIDR }}
    configSyncPeriod: 0s
    featureGates:
      ServiceInternalTrafficPolicy: true
    {{ if .DualStack }}
      IPv6DualStack: true
    {{ end }}
    mode: "{{ .Mode }}"
    conntrack:
      maxPerCore: 0
      min: null
      tcpCloseWaitTimeout: null
      tcpEstablishedTimeout: null
    detectLocalMode: ""
    enableProfiling: false
    healthzBindAddress: ""
    hostnameOverride: ""
    iptables:
      masqueradeAll: false
      masqueradeBit: null
      minSyncPeriod: 0s
      syncPeriod: 0s
    ipvs:
      excludeCIDRs: null
      minSyncPeriod: 0s
      scheduler: ""
      strictARP: false
      syncPeriod: 0s
      tcpFinTimeout: 0s
      tcpTimeout: 0s
      udpTimeout: 0s
    kind: KubeProxyConfiguration
    metricsBindAddress: ""
    nodePortAddresses: null
    oomScoreAdj: null
    portRange: ""
    showHiddenMetricsForVersion: ""
    udpIdleTimeout: 0s
    winkernel:
      enableDSR: false
      networkName: ""
      sourceVip: ""
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    k8s-app: kube-proxy
  name: kube-proxy
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: kube-proxy
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        k8s-app: kube-proxy
    spec:
      priorityClassName: system-node-critical
      containers:
      - name: kube-proxy
        image: {{ .Image }}
        imagePullPolicy: {{ .PullPolicy }}
        command:
        - /usr/local/bin/kube-proxy
        - --config=/var/lib/kube-proxy/config.conf
        - --hostname-override=$(NODE_NAME)
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /var/lib/kube-proxy
          name: kube-proxy
        - mountPath: /run/xtables.lock
          name: xtables-lock
          readOnly: false
        - mountPath: /lib/modules
          name: lib-modules
          readOnly: true
        env:
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
      hostNetwork: true
      serviceAccountName: kube-proxy
      volumes:
      - name: kube-proxy
        configMap:
          name: kube-proxy
      - name: xtables-lock
        hostPath:
          path: /run/xtables.lock
          type: FileOrCreate
      - name: lib-modules
        hostPath:
          path: /lib/modules
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      - operator: Exists
      - key: "node-role.kubernetes.io/master"
        operator: "Exists"
        effect: "NoSchedule"
      nodeSelector:
        kubernetes.io/os: linux
`

// Health-check interface
func (k *KubeProxy) Healthy() error { return nil }
