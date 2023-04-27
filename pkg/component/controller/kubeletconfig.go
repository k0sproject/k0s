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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"reflect"

	"github.com/imdario/mergo"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Dummy checks so we catch easily if we miss some interface implementation
var _ manager.Reconciler = (*KubeletConfig)(nil)
var _ manager.Component = (*KubeletConfig)(nil)

// KubeletConfig is the reconciler for generic kubelet configs
type KubeletConfig struct {
	log logrus.FieldLogger

	kubeClientFactory k8sutil.ClientFactoryInterface
	k0sVars           *config.CfgVars
	previousProfiles  v1beta1.WorkerProfiles
	nodeConfig        *v1beta1.ClusterConfig
}

// NewKubeletConfig creates new KubeletConfig reconciler
func NewKubeletConfig(k0sVars *config.CfgVars, clientFactory k8sutil.ClientFactoryInterface, nodeConfig *v1beta1.ClusterConfig) *KubeletConfig {
	return &KubeletConfig{
		log: logrus.WithFields(logrus.Fields{"component": "kubeletconfig"}),

		kubeClientFactory: clientFactory,
		k0sVars:           k0sVars,
		nodeConfig:        nodeConfig,
	}
}

// Init does nothing
func (k *KubeletConfig) Init(_ context.Context) error {
	return nil
}

// Stop does nothign, nothing actually running
func (k *KubeletConfig) Stop() error {
	return nil
}

// Run dumps the needed manifest objects
func (k *KubeletConfig) Start(_ context.Context) error {

	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (k *KubeletConfig) Reconcile(ctx context.Context, clusterSpec *v1beta1.ClusterConfig) error {
	k.log.Debug("reconcile method called for: KubeletConfig")
	// Check if we actually need to reconcile anything
	defaultProfilesExist, err := k.defaultProfilesExist(ctx)
	if err != nil {
		return err
	}
	if defaultProfilesExist && reflect.DeepEqual(k.previousProfiles, clusterSpec.Spec.WorkerProfiles) {
		k.log.Debugf("default profiles exist and no change in user specified profiles, nothing to reconcile")
		return nil
	}

	manifest, err := k.createProfiles(clusterSpec)
	if err != nil {
		return fmt.Errorf("failed to build final manifest: %v", err)
	}

	if err := k.save(manifest.Bytes()); err != nil {
		return fmt.Errorf("can't write manifest with config maps: %v", err)
	}
	k.previousProfiles = clusterSpec.Spec.WorkerProfiles

	return nil
}

func (k *KubeletConfig) defaultProfilesExist(ctx context.Context) (bool, error) {
	c, err := k.kubeClientFactory.GetClient()
	if err != nil {
		return false, err
	}
	defaultProfileName := formatProfileName("default")
	_, err = c.CoreV1().ConfigMaps("kube-system").Get(ctx, defaultProfileName, v1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (k *KubeletConfig) createProfiles(clusterSpec *v1beta1.ClusterConfig) (*bytes.Buffer, error) {
	dnsAddress, err := k.nodeConfig.Spec.Network.DNSAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS address for kubelet config: %v", err)
	}
	manifest := bytes.NewBuffer([]byte{})
	defaultProfile := getDefaultProfile(dnsAddress, clusterSpec.Spec.Network.ClusterDomain)
	defaultProfile["cgroupsPerQOS"] = true

	winDefaultProfile := getDefaultProfile(dnsAddress, clusterSpec.Spec.Network.ClusterDomain)
	winDefaultProfile["cgroupsPerQOS"] = false

	if err := k.writeConfigMapWithProfile(manifest, "default", defaultProfile); err != nil {
		return nil, fmt.Errorf("can't write manifest for default profile config map: %v", err)
	}
	if err := k.writeConfigMapWithProfile(manifest, "default-windows", winDefaultProfile); err != nil {
		return nil, fmt.Errorf("can't write manifest for default profile config map: %v", err)
	}
	configMapNames := []string{
		formatProfileName("default"),
		formatProfileName("default-windows"),
	}
	for _, profile := range clusterSpec.Spec.WorkerProfiles {
		profileConfig := getDefaultProfile(dnsAddress, clusterSpec.Spec.Network.ClusterDomain)

		var workerValues unstructuredYamlObject
		err := json.Unmarshal(profile.Config, &workerValues)
		if err != nil {
			return nil, fmt.Errorf("failed to decode worker profile values: %v", err)
		}
		merged, err := mergeProfiles(&profileConfig, workerValues)
		if err != nil {
			return nil, fmt.Errorf("can't merge profile `%s` with default profile: %v", profile.Name, err)
		}

		if err := k.writeConfigMapWithProfile(manifest,
			profile.Name,
			merged); err != nil {
			return nil, fmt.Errorf("can't write manifest for profile config map: %v", err)
		}
		configMapNames = append(configMapNames, formatProfileName(profile.Name))
	}
	if err := k.writeRbacRoleBindings(manifest, configMapNames); err != nil {
		return nil, fmt.Errorf("can't write manifest for rbac bindings: %v", err)
	}
	return manifest, nil
}

func (k *KubeletConfig) save(data []byte) error {
	kubeletDir := path.Join(k.k0sVars.ManifestsDir, "kubelet")
	err := dir.Init(kubeletDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	filePath := filepath.Join(kubeletDir, "kubelet-config.yaml")
	if err := file.WriteContentAtomically(filePath, data, constant.CertMode); err != nil {
		return fmt.Errorf("can't write kubelet configuration config map: %v", err)
	}

	deprecationNotice := []byte(`The kubelet-config component has been replaced by the worker-config component in k0s 1.26.
It is scheduled for removal in k0s 1.27.
`)

	if err := file.WriteContentAtomically(filepath.Join(kubeletDir, "deprecated.txt"), deprecationNotice, constant.CertMode); err != nil {
		k.log.WithError(err).Warn("Failed to write deprecation notice")
	}

	return nil
}

type unstructuredYamlObject map[string]interface{}

func (k *KubeletConfig) writeConfigMapWithProfile(w io.Writer, name string, profile unstructuredYamlObject) error {
	profileYaml, err := yaml.Marshal(profile)
	if err != nil {
		return err
	}
	tw := templatewriter.TemplateWriter{
		Name:     "kubelet-config",
		Template: kubeletConfigsManifestTemplate,
		Data: struct {
			Name              string
			KubeletConfigYAML string
		}{
			Name:              formatProfileName(name),
			KubeletConfigYAML: string(profileYaml),
		},
	}
	return tw.WriteToBuffer(w)
}

func formatProfileName(name string) string {
	return fmt.Sprintf("kubelet-config-%s-%s", name, constant.KubernetesMajorMinorVersion)
}

func (k *KubeletConfig) writeRbacRoleBindings(w io.Writer, configMapNames []string) error {
	tw := templatewriter.TemplateWriter{
		Name:     "kubelet-config-rbac",
		Template: rbacRoleAndBindingsManifestTemplate,
		Data: struct {
			ConfigMapNames []string
		}{
			ConfigMapNames: configMapNames,
		},
	}

	return tw.WriteToBuffer(w)
}

func getDefaultProfile(dnsAddress string, clusterDomain string) unstructuredYamlObject {
	// the motivation to keep it like this instead of the yaml template:
	// - it's easier to merge programatically defined structure
	// - apart from map[string]interface there is no good way to define free-form mapping

	cipherSuites := make([]string, len(constant.AllowedTLS12CipherSuiteIDs))
	for i, cipherSuite := range constant.AllowedTLS12CipherSuiteIDs {
		cipherSuites[i] = tls.CipherSuiteName(cipherSuite)
	}

	// for the authentication.x509.clientCAFile and volumePluginDir we want to use later binding so we put template placeholder instead of actual value there
	profile := unstructuredYamlObject{
		"apiVersion":         "kubelet.config.k8s.io/v1beta1",
		"kind":               "KubeletConfiguration",
		"clusterDNS":         []string{dnsAddress},
		"clusterDomain":      clusterDomain,
		"tlsCipherSuites":    cipherSuites,
		"failSwapOn":         false,
		"rotateCertificates": true,
		"serverTLSBootstrap": true,
		"eventRecordQPS":     0,
	}
	return profile
}

const kubeletConfigsManifestTemplate = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.Name}}
  namespace: kube-system
  labels:
    k0s.k0sproject.io/deprecated-since: "1.26"
  annotations:
    k0s.k0sproject.io/deprecated: |
      The kubelet-config component has been replaced by the worker-config component in k0s 1.26.
      It is scheduled for removal in k0s 1.27.
data:
  kubelet: |
{{ .KubeletConfigYAML | nindent 4 }}
`

const rbacRoleAndBindingsManifestTemplate = `---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: system:bootstrappers:kubelet-configmaps
  namespace: kube-system
  labels:
    k0s.k0sproject.io/deprecated-since: "1.26"
  annotations:
    k0s.k0sproject.io/deprecated: |
      The kubelet-config component has been replaced by the worker-config component in k0s 1.26.
      It is scheduled for removal in k0s 1.27.
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  resourceNames:
{{- range .ConfigMapNames }}
    - "{{ . -}}"
{{ end }}
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system:bootstrappers:kubelet-configmaps
  namespace: kube-system
  labels:
    k0s.k0sproject.io/deprecated-since: "1.26"
  annotations:
    k0s.k0sproject.io/deprecated: |
      The kubelet-config component has been replaced by the worker-config component in k0s 1.26.
      It is scheduled for removal in k0s 1.27.
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: system:bootstrappers:kubelet-configmaps
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: Group
    name: system:bootstrappers
  - apiGroup: rbac.authorization.k8s.io
    kind: Group
    name: system:nodes
`

// mergeInto merges b to the a, a is modified inplace
func mergeProfiles(a *unstructuredYamlObject, b unstructuredYamlObject) (unstructuredYamlObject, error) {
	if err := mergo.Merge(a, b, mergo.WithOverride); err != nil {
		return nil, err
	}
	return *a, nil
}
