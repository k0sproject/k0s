// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

// KubeProxy is the component implementation to manage kube-proxy
type KubeProxy struct {
	log logrus.FieldLogger

	nodeConf    *v1beta1.ClusterConfig
	K0sVars     *config.CfgVars
	manifestDir string

	previousConfig proxyConfig
}

var _ manager.Component = (*KubeProxy)(nil)
var _ manager.Reconciler = (*KubeProxy)(nil)

// NewKubeProxy creates new KubeProxy component
func NewKubeProxy(k0sVars *config.CfgVars, nodeConfig *v1beta1.ClusterConfig) *KubeProxy {
	return &KubeProxy{
		log: logrus.WithFields(logrus.Fields{"component": "kubeproxy"}),

		nodeConf:    nodeConfig,
		K0sVars:     k0sVars,
		manifestDir: path.Join(k0sVars.ManifestsDir, "kubeproxy"),
	}
}

// Init does nothing
func (k *KubeProxy) Init(_ context.Context) error {
	return nil
}

// Run runs the kube-proxy reconciler
func (k *KubeProxy) Start(_ context.Context) error { return nil }

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
	if reflect.DeepEqual(cfg, k.previousConfig) {
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
	controlPlaneEndpoint := k.nodeConf.Spec.API.APIAddressURL()
	nllb := clusterConfig.Spec.Network.NodeLocalLoadBalancing
	if nllb.IsEnabled() {
		switch nllb.Type {
		case v1beta1.NllbTypeEnvoyProxy:
			k.log.Debugf("Enabling node-local load balancing via %s", nllb.Type)

			// FIXME: Transitions from non-node-local load balanced to node-local load
			// balanced setups will be problematic: The controller will update the
			// DaemonSet with localhost, but the worker nodes won't reconcile their
			// state (yet) and need to be restarted manually in order to start their
			// load balancer. Transitions in the other direction suffer from the same
			// limitation, but that will be less grave, as the node-local load
			// balancers will remain operational until the next node restart and the
			// proxy will stay connected.

			// FIXME: This is not exactly on par with the way it's implemented on the
			// worker side, i.e. there's no fallback if localhost doesn't resolve to a
			// loopback address. But this would require some shenanigans to pull in
			// node-specific values here. A possible solution would be to convert
			// kube-proxy to a static Pod as well.
			controlPlaneEndpoint = fmt.Sprintf("https://localhost:%d", nllb.EnvoyProxy.APIServerBindPort)

		default:
			k.log.Warnf("Unsupported node-local load balancer type (%q), using %q as control plane endpoint", controlPlaneEndpoint)
		}
	}
	args := stringmap.StringMap{
		"config":            "/var/lib/kube-proxy/config.conf",
		"hostname-override": "$(NODE_NAME)",
	}

	for name, value := range clusterConfig.Spec.Network.KubeProxy.ExtraArgs {
		if _, ok := args[name]; ok {
			logrus.Warnf("overriding kube-proxy flag with user provided value: %s", name)
		}
		args[name] = value
	}

	cfg := proxyConfig{
		ClusterCIDR:          clusterConfig.Spec.Network.BuildPodCIDR(),
		ControlPlaneEndpoint: controlPlaneEndpoint,
		Image:                clusterConfig.Spec.Images.KubeProxy.URI(),
		PullPolicy:           clusterConfig.Spec.Images.DefaultPullPolicy,
		DualStack:            clusterConfig.Spec.Network.DualStack.Enabled,
		Mode:                 clusterConfig.Spec.Network.KubeProxy.Mode,
		MetricsBindAddress:   clusterConfig.Spec.Network.KubeProxy.MetricsBindAddress,
		FeatureGates:         clusterConfig.Spec.FeatureGates.AsMap("kube-proxy"),
		Args:                 args.ToDashedArgs(),
	}

	nodePortAddresses, err := json.Marshal(clusterConfig.Spec.Network.KubeProxy.NodePortAddresses)
	if err != nil {
		return proxyConfig{}, err
	}
	cfg.NodePortAddresses = string(nodePortAddresses)

	iptables, err := json.Marshal(clusterConfig.Spec.Network.KubeProxy.IPTables)
	if err != nil {
		return proxyConfig{}, err
	}
	cfg.IPTables = string(iptables)

	ipvs, err := json.Marshal(clusterConfig.Spec.Network.KubeProxy.IPVS)
	if err != nil {
		return proxyConfig{}, err
	}
	cfg.IPVS = string(ipvs)

	nftables, err := json.Marshal(clusterConfig.Spec.Network.KubeProxy.NFTables)
	if err != nil {
		return proxyConfig{}, err
	}
	cfg.NFTables = string(nftables)

	return cfg, nil
}

type proxyConfig struct {
	DualStack            bool
	ControlPlaneEndpoint string
	ClusterCIDR          string
	Image                string
	PullPolicy           string
	Mode                 string
	MetricsBindAddress   string
	IPTables             string
	IPVS                 string
	NFTables             string
	FeatureGates         map[string]bool
	NodePortAddresses    string
	Args                 []string
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
{{- range $key, $value := .FeatureGates }}
      {{ $key }}: {{ $value }}
{{- end }}
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
    iptables: {{ .IPTables }}
    ipvs: {{ .IPVS }}
    nftables: {{ .NFTables }}
    kind: KubeProxyConfiguration
    metricsBindAddress: {{ .MetricsBindAddress }}
    nodePortAddresses: {{ .NodePortAddresses }}
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
      annotations:
        prometheus.io/scrape: 'true'
        prometheus.io/port: '10249'
    spec:
      priorityClassName: system-node-critical
      containers:
      - name: kube-proxy
        image: {{ .Image }}
        imagePullPolicy: {{ .PullPolicy }}
        command:
        - /usr/local/bin/kube-proxy
        args:
        {{ range .Args}}
        - {{ . }} 
        {{ end }}
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
      - operator: Exists
        effect: NoExecute
      - operator: Exists
        effect: NoSchedule
      nodeSelector:
        kubernetes.io/os: linux
`
