package server

import (
	"bytes"
	"fmt"
	"path/filepath"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
)

// KubeletConfig is the reconciler for generic kubelet configs
type KubeletConfig struct {
	clusterSpec *config.ClusterSpec
	log         *logrus.Entry
}

// NewKubeletConfig creates new KubeletConfig reconciler
func NewKubeletConfig(clusterSpec *config.ClusterSpec) (*KubeletConfig, error) {
	log := logrus.WithFields(logrus.Fields{"component": "kubeletconfig"})
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
		return fmt.Errorf("failed to get DNS address for kubelet config: %v", err)
	}

	manifest := bytes.NewBuffer([]byte{})
	defaultProfile := k.getDefaultProfile(dnsAddress)

	if err := k.writeConfigMapWithProfile(manifest, "default", defaultProfile); err != nil {
		return fmt.Errorf("can't write manifest for default profile config map: %v", err)
	}

	for _, profile := range k.clusterSpec.WorkerProfiles {
		profileConfig := k.getDefaultProfile(dnsAddress)
		if err := k.mergeProfiles(&profileConfig,
			profile.Values); err != nil {
			return fmt.Errorf("can't merge profile `%s` with default profile: %v", profile.Name, err)
		}

		if err := k.writeConfigMapWithProfile(manifest,
			profile.Name,
			profileConfig); err != nil {
			return fmt.Errorf("can't write manifest for profile config map: %v", err)
		}
	}

	if err := k.save(manifest.Bytes()); err != nil {
		return fmt.Errorf("can't write manifest with config maps: %v", err)
	}

	return nil
}

func (k *KubeletConfig) save(data []byte) error {
	path := filepath.Join(constant.ManifestsDir, "kubelet", "kubelet-config.yaml")

	if err := ioutil.WriteFile(path, data, 0700); err != nil {
		return fmt.Errorf("can't write kubelet configuration config map: %v", err)
	}
	return nil
}

type unstructuredYamlObject map[string]interface{}

func (k *KubeletConfig) writeConfigMapWithProfile(w io.Writer, name string, profile unstructuredYamlObject) error {
	profileYaml, err := yaml.Marshal(profile)
	if err != nil {
		return err
	}
	tw := util.TemplateWriter{
		Name:     "kubelet-config",
		Template: kubeletConfigsManifestTemplate,
		Data: struct {
			Name              string
			KubeletConfigYAML string
		}{
			Name:              fmt.Sprintf("kubelet-config-%s-%s", name, constant.KubernetesMajorMinorVersion),
			KubeletConfigYAML: string(profileYaml),
		},
	}
	return tw.WriteToBuffer(w)
}

// mergeProfiles merges b to the a, a is modified inplace
func (k *KubeletConfig) mergeProfiles(a *unstructuredYamlObject, b unstructuredYamlObject) error {
	return mergo.Merge(a, b, mergo.WithOverride)
}

func (l *KubeletConfig) getDefaultProfile(dnsAddess string) unstructuredYamlObject {
	// the motivation to keep it like this instead of the yaml template:
	// - it's easier to merge programatically defined structure
	// - apart from map[string]interface there is no good way to define free-form mapping
	// - another good options is to use "k8s.io/kubelet/config/v1beta1" package directly
	return unstructuredYamlObject{
		"apiVersion": "kubelet.config.k8s.io/v1beta1",
		"kind":       "KubeletConfiguration",
		"authentication": map[string]interface{}{
			"anonymous": map[string]interface{}{
				"enabled": false,
			},
			"webhook": map[string]interface{}{
				"cacheTTL": "0s",
				"enabled":  true,
			},
			"x509": map[string]interface{}{
				"clientCAFile": "/var/lib/mke/pki/ca.crt",
			},
		},
		"authorization": map[string]interface{}{
			"mode": "Webhook",
			"webhook": map[string]interface{}{
				"cacheAuthorizedTTL":   "0s",
				"cacheUnauthorizedTTL": "0s",
			},
		},
		"clusterDNS":    []string{dnsAddess},
		"clusterDomain": "cluster.local",
		"tlsCipherSuites": []string{
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_RSA_WITH_AES_128_GCM_SHA256",
		},
		"volumeStatsAggPeriod": "0s",
		"failSwapOn":           false,
	}
}

const kubeletConfigsManifestTemplate = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.Name}}
  namespace: kube-system
data:
  kubelet: | 
{{ .KubeletConfigYAML |indent 4 }}
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
