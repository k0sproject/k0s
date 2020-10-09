package server

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// KubeletConfig is the reconciler for generic kubelet configs
type KubeletConfig struct {
	clusterSpec *config.ClusterSpec
	log         *logrus.Entry
}

// NewKubeletConfig creates new KubeletConfig reconciler
func NewKubeletConfig(clusterSpec *config.ClusterSpec) (*KubeletConfig, error) {

	log := logrus.WithFields(logrus.Fields{"component": "kubelectconfig"})
	return &KubeletConfig{
		log:         log,
		clusterSpec: clusterSpec,
	}, nil
}

// Init does nothing
func (k *KubeletConfig) Init() error {
	return nil
}

// Stop does nothign, nothing actually running
func (k *KubeletConfig) Stop() error {
	return nil
}

// Run dumps the needed manifest objects
func (k *KubeletConfig) Run() error {
	dnsAddress, err := k.clusterSpec.Network.DNSAddress()
	if err != nil {
		return errors.Wrap(err, "failed to get DNS address for kubelet config")
	}
	config := struct {
		Name          string
		ClusterDNS    string
		ClusterDomain string
	}{
		Name:          fmt.Sprintf("kubelet-config-%s", constant.KubernetesMajorMinorVersion),
		ClusterDNS:    dnsAddress,
		ClusterDomain: "cluster.local",
	}

	kubeletDir := path.Join(constant.ManifestsDir, "kubeletconfig")
	err = os.MkdirAll(kubeletDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	tw := util.TemplateWriter{
		Name:     "kubelet-config",
		Template: kubeletConfigTemplate,
		Data:     config,
		Path:     filepath.Join(kubeletDir, "kubelet-config.yaml"),
	}
	err = tw.Write()
	if err != nil {
		return errors.Wrap(err, "error writing kubelet-config manifests, will NOT retry")

	}
	return nil
}

const kubeletConfigTemplate = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.Name}}
  namespace: kube-system
data:
  kubelet: |
    apiVersion: kubelet.config.k8s.io/v1beta1
    kind: KubeletConfiguration
    authentication:
      anonymous:
        enabled: false
      webhook:
        cacheTTL: 0s
        enabled: true
      x509:
        clientCAFile: /var/lib/mke/pki/ca.crt
    authorization:
      mode: Webhook
      webhook:
        cacheAuthorizedTTL: 0s
        cacheUnauthorizedTTL: 0s
    clusterDNS:
    - "{{ .ClusterDNS }}"
    clusterDomain: "{{ .ClusterDomain }}"
    tlsCipherSuites:
    - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
    - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
    - TLS_RSA_WITH_AES_256_GCM_SHA384
    - TLS_RSA_WITH_AES_128_GCM_SHA256
    volumeStatsAggPeriod: 0s
    failSwapOn: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: system:bootstrappers:{{.Name}}
  namespace: kube-system
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  resourceNames: ["{{ .Name }}"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system:bootstrappers:{{.Name}}
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: system:bootstrappers:{{.Name}}
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: Group
    name: system:bootstrappers
  - apiGroup: rbac.authorization.k8s.io
    kind: Group
    name: system:nodes
`
