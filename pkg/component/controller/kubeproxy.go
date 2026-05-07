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
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
	kubeproxyv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
	"k8s.io/utils/ptr"

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
		manifestDir: filepath.Join(k0sVars.ManifestsDir, "kubeproxy"),
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
	cfg := k.getConfig(clusterConfig)
	if reflect.DeepEqual(cfg, k.previousConfig) {
		k.log.Infof("current cfg matches existing, not gonna do anything")
		return nil
	}
	err = file.AtomicWithTarget(filepath.Join(k.manifestDir, "kube-proxy.yaml")).
		Do(func(unbuffered file.AtomicWriter) error {
			buf := bufio.NewWriter(unbuffered)

			templateData := struct {
				*kubeProxyTemplateData
				ConfigHash string
			}{
				kubeProxyTemplateData: &cfg.TemplateData,
			}

			if configMap, err := cfg.ConfigMapData.toConfigMap(); err != nil {
				return err
			} else {
				hasher := sha256.New()
				buf := io.MultiWriter(hasher, buf)
				codec := applier.CodecFor(kubernetesscheme.Scheme)
				if err := codec.Encode(configMap, buf); err != nil {
					return err
				}
				templateData.ConfigHash = base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
			}

			if err := (&templatewriter.TemplateWriter{
				Name:     "kube-proxy",
				Template: proxyTemplate,
				Data:     &templateData,
			}).WriteToBuffer(buf); err != nil {
				return err
			}

			return buf.Flush()
		})
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

func (k *KubeProxy) getConfig(clusterConfig *v1beta1.ClusterConfig) proxyConfig {
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

	kubeProxy := clusterConfig.Spec.Network.KubeProxy
	for name, value := range kubeProxy.ExtraArgs {
		if _, ok := args[name]; ok {
			logrus.Warnf("overriding kube-proxy flag with user provided value: %s", name)
		}
		args[name] = value
	}

	return proxyConfig{
		TemplateData: kubeProxyTemplateData{
			Image:      clusterConfig.Spec.Images.KubeProxy.URI(),
			PullPolicy: clusterConfig.Spec.Images.DefaultPullPolicy,
			Args:       args.ToDashedArgs(),
		},
		ConfigMapData: kubeProxyConfigData{
			apiServerEndpoint: controlPlaneEndpoint,
			config: kubeproxyv1alpha1.KubeProxyConfiguration{
				ClientConnection: configv1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "/var/lib/kube-proxy/kubeconfig.conf",
				},
				ClusterCIDR:        clusterConfig.Spec.Network.BuildPodCIDR(),
				FeatureGates:       clusterConfig.Spec.FeatureGates.AsMap("kube-proxy"),
				Mode:               kubeproxyv1alpha1.ProxyMode(kubeProxy.Mode),
				MetricsBindAddress: kubeProxy.MetricsBindAddress,
				Conntrack: kubeproxyv1alpha1.KubeProxyConntrackConfiguration{
					MaxPerCore: ptr.To(int32(0)),
				},
				IPTables: kubeproxyv1alpha1.KubeProxyIPTablesConfiguration{
					MasqueradeBit:      kubeProxy.IPTables.MasqueradeBit,
					MasqueradeAll:      kubeProxy.IPTables.MasqueradeAll,
					LocalhostNodePorts: kubeProxy.IPTables.LocalhostNodePorts,
					SyncPeriod:         kubeProxy.IPTables.SyncPeriod,
					MinSyncPeriod:      kubeProxy.IPTables.MinSyncPeriod,
				},
				IPVS: kubeproxyv1alpha1.KubeProxyIPVSConfiguration{
					SyncPeriod:    kubeProxy.IPVS.SyncPeriod,
					MinSyncPeriod: kubeProxy.IPVS.MinSyncPeriod,
					Scheduler:     kubeProxy.IPVS.Scheduler,
					ExcludeCIDRs:  kubeProxy.IPVS.ExcludeCIDRs,
					StrictARP:     kubeProxy.IPVS.StrictARP,
					TCPTimeout:    kubeProxy.IPVS.TCPTimeout,
					TCPFinTimeout: kubeProxy.IPVS.TCPFinTimeout,
					UDPTimeout:    kubeProxy.IPVS.UDPTimeout,
				},
				NFTables: kubeproxyv1alpha1.KubeProxyNFTablesConfiguration{
					SyncPeriod:    kubeProxy.NFTables.SyncPeriod,
					MasqueradeBit: kubeProxy.NFTables.MasqueradeBit,
					MasqueradeAll: kubeProxy.NFTables.MasqueradeAll,
					MinSyncPeriod: kubeProxy.NFTables.MinSyncPeriod,
				},
				NodePortAddresses: kubeProxy.NodePortAddresses,
			},
		},
	}
}

type proxyConfig struct {
	TemplateData  kubeProxyTemplateData
	ConfigMapData kubeProxyConfigData
}

type kubeProxyTemplateData struct {
	Image      string
	PullPolicy string
	Args       []string
}

type kubeProxyConfigData struct {
	apiServerEndpoint string
	config            kubeproxyv1alpha1.KubeProxyConfiguration
}

func (d *kubeProxyConfigData) toConfigMap() (*corev1.ConfigMap, error) {
	codec := applier.CodecFor(applier.BuildScheme(kubeproxyv1alpha1.AddToScheme))

	const (
		clusterName = "local"
		contextName = "default"
		userName    = "user"
	)

	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{clusterName: {
			Server:               d.apiServerEndpoint,
			CertificateAuthority: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		}},
		Contexts: map[string]*clientcmdapi.Context{contextName: {
			Cluster:  clusterName,
			AuthInfo: userName,
		}},
		CurrentContext: contextName,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{userName: {
			TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		}},
	})
	if err != nil {
		return nil, err
	}

	var config strings.Builder
	if err := codec.Encode(&d.config, &config); err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem, Name: "kube-proxy",
			Labels: map[string]string{"k8s-app": "kube-proxy"},
		},
		Data: map[string]string{
			"kubeconfig.conf": string(kubeconfig),
			"config.conf":     config.String(),
		},
	}, nil
}

const proxyTemplate = `
---
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
        k0sproject.io/config-hash: {{ .ConfigHash }}
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
