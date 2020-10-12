package server

import (
	"os"
	"path"
	"path/filepath"
	"time"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/sirupsen/logrus"
)

// KubeProxy is the compoennt implementation to manage kube-proxy
type KubeProxy struct {
	//client     *kubernetes.Clientset
	tickerDone  chan struct{}
	log         *logrus.Entry
	clusterSpec *config.ClusterSpec
}

// NewKubeProxy creates new KubeProxy component
func NewKubeProxy(clusterSpec *config.ClusterSpec) (*KubeProxy, error) {

	log := logrus.WithFields(logrus.Fields{"component": "kubeproxy"})
	return &KubeProxy{
		log:         log,
		clusterSpec: clusterSpec,
	}, nil
}

// Init does nothing
func (k *KubeProxy) Init() error {
	return nil
}

// Run runs the kube-proxy reconciler
func (k *KubeProxy) Run() error {

	k.tickerDone = make(chan struct{})

	proxyDir := path.Join(constant.ManifestsDir, "kubeproxy")
	err := os.MkdirAll(proxyDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		var previousConfig = proxyConfig{}
		for {
			select {
			case <-ticker.C:
				config, err := k.getConfig()
				if err != nil {
					k.log.Errorf("error calculating proxy configs: %s. will retry", err.Error())
					continue
				}
				if config == previousConfig {
					k.log.Infof("current config matches existing, not gonna do anything")
					continue
				}
				tw := util.TemplateWriter{
					Name:     "kube-proxy",
					Template: proxyTemplate,
					Data:     config,
					Path:     filepath.Join(proxyDir, "kube-proxy.yaml"),
				}
				err = tw.Write()
				if err != nil {
					k.log.Errorf("error writing kube-proxy manifests: %s. will retry", err.Error())
					continue
				}
				previousConfig = config
			case <-k.tickerDone:
				k.log.Info("proxy reconciler done")
				return
			}
		}
	}()

	return nil
}

// Stop stop the reconcilier
func (k *KubeProxy) Stop() error {
	close(k.tickerDone)
	return nil
}

func (k *KubeProxy) getConfig() (proxyConfig, error) {
	config := proxyConfig{
		// FIXME get this from somewhere
		ControlPlaneEndpoint: k.clusterSpec.API.APIAddress(),
		ClusterCIDR:          k.clusterSpec.Network.PodCIDR,
	}

	return config, nil
}

type proxyConfig struct {
	ControlPlaneEndpoint string
	ClusterCIDR          string
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
apiVersion: rbac.authorization.k8s.io/v1beta1
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
apiVersion: rbac.authorization.k8s.io/v1beta1
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
    mode: ""
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
        image: k8s.gcr.io/kube-proxy:v1.19.0
        imagePullPolicy: IfNotPresent
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
      nodeSelector:
        kubernetes.io/os: linux
`
