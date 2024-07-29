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

package v1beta1

import (
	"encoding/json"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/iface"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestClusterDefaults(t *testing.T) {
	c, err := ConfigFromString("apiVersion: k0s.k0sproject.io/v1beta1")
	assert.NoError(t, err)
	assert.Equal(t, DefaultStorageSpec(), c.Spec.Storage)
}

func TestUnknownFieldValidation(t *testing.T) {
	_, err := ConfigFromString(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
unknown: 1`)

	assert.Error(t, err)
}

func TestStorageDefaults(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, EtcdStorageType, c.Spec.Storage.Type)
	addr, err := iface.FirstPublicAddress()
	assert.NoError(t, err)
	assert.Equal(t, addr, c.Spec.Storage.Etcd.PeerAddress)
}

func TestEmptyClusterSpec(t *testing.T) {
	underTest := ClusterConfig{
		Spec: &ClusterSpec{},
	}

	errs := underTest.Validate()
	assert.Nil(t, errs)
}

func TestClusterSpecCustomImages(t *testing.T) {
	defaultTestCase := ClusterConfig{
		Spec: &ClusterSpec{
			Images: DefaultClusterImages(),
		},
	}
	errs := defaultTestCase.Validate()
	assert.Nilf(t, errs, "%v", errs)

	validTestCase := ClusterConfig{
		Spec: &ClusterSpec{
			Images: DefaultClusterImages(),
		},
	}
	validTestCase.Spec.Images.DefaultPullPolicy = string(corev1.PullIfNotPresent)
	validTestCase.Spec.Images.Konnectivity = ImageSpec{
		Image:   "foo",
		Version: "v1",
	}
	validTestCase.Spec.Images.PushGateway = ImageSpec{
		Image:   "bar",
		Version: "v2@sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}

	errs = validTestCase.Validate()
	assert.Nilf(t, errs, "%v", errs)

	invalidTestCase := ClusterConfig{
		Spec: &ClusterSpec{
			Images: DefaultClusterImages(),
		},
	}
	invalidTestCase.Spec.Images.MetricsServer = ImageSpec{
		Image: "baz",
		// digest only is currently not supported
		Version: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	invalidTestCase.Spec.Images.Calico.CNI = ImageSpec{
		Image: "qux",
		// digest only is currently not supported
		Version: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	invalidTestCase.Spec.Images.KubeRouter.CNI = ImageSpec{
		Image: "quux",
		// digest only is currently not supported
		Version: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}

	errs = invalidTestCase.Validate()
	assert.Len(t, errs, 3)
}

func TestEtcdDefaults(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  storage:
    type: etcd
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, EtcdStorageType, c.Spec.Storage.Type)
	addr, err := iface.FirstPublicAddress()
	assert.NoError(t, err)
	assert.Equal(t, addr, c.Spec.Storage.Etcd.PeerAddress)
}

func TestNetworkValidation_Custom(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: custom
  storage:
    type: etcd
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Equal(t, 0, len(errors))
}

func TestNetworkValidation_Calico(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: calico
  storage:
    type: etcd
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Equal(t, 0, len(errors))
}

func TestNetworkValidation_Invalid(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: invalidProvider
  storage:
    type: etcd
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	if assert.Len(t, errors, 1) {
		assert.ErrorContains(t, errors[0], `spec: network: provider: Unsupported value: "invalidProvider": supported values: "kuberouter", "calico", "custom"`)
	}
}

func TestApiExternalAddress(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  api:
    externalAddress: foo.bar.com
    address: 1.2.3.4
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "https://foo.bar.com:6443", c.Spec.API.APIAddressURL())
	assert.Equal(t, "https://foo.bar.com:9443", c.Spec.API.K0sControlPlaneAPIAddress())
}

func TestApiNoExternalAddress(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  api:
    address: 1.2.3.4
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "https://1.2.3.4:6443", c.Spec.API.APIAddressURL())
	assert.Equal(t, "https://1.2.3.4:9443", c.Spec.API.K0sControlPlaneAPIAddress())
}

func TestNullValues(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  images: null
  storage: null
  network: null
  api: null
  extensions: null
  controllerManager: null
  scheduler: null
  installConfig: null
  telemetry: null
  konnectivity: null
`
	extensionsYamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  extensions:
    storage: null
`

	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, DefaultClusterImages(), c.Spec.Images)
	assert.Equal(t, DefaultStorageSpec(), c.Spec.Storage)
	assert.Equal(t, DefaultNetwork(), c.Spec.Network)
	assert.Equal(t, DefaultAPISpec(), c.Spec.API)
	assert.Equal(t, DefaultExtensions(), c.Spec.Extensions)
	assert.Equal(t, DefaultStorageSpec(), c.Spec.Storage)
	assert.Equal(t, DefaultControllerManagerSpec(), c.Spec.ControllerManager)
	assert.Equal(t, DefaultSchedulerSpec(), c.Spec.Scheduler)
	assert.Equal(t, DefaultInstallSpec(), c.Spec.Install)
	assert.Equal(t, DefaultClusterTelemetry(), c.Spec.Telemetry)
	assert.Equal(t, DefaultKonnectivitySpec(), c.Spec.Konnectivity)

	e, err := ConfigFromString(extensionsYamlData)
	assert.NoError(t, err)
	assert.Equal(t, DefaultExtensions(), e.Spec.Extensions)
}

func TestWorkerProfileConfig(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  workerProfiles:
  - name: profile_XXX
    values:
      authentication:
        anonymous:
          enabled: true
        webhook:
          cacheTTL: 2m0s
          enabled: true
  - name: profile_YYY
    values:
      apiVersion: v2
      authentication:
        anonymous:
          enabled: false
`
	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(c.Spec.WorkerProfiles))
	assert.Equal(t, "profile_XXX", c.Spec.WorkerProfiles[0].Name)
	assert.Equal(t, "profile_YYY", c.Spec.WorkerProfiles[1].Name)

	j := c.Spec.WorkerProfiles[1].Config
	var parsed map[string]interface{}

	err = json.Unmarshal(j.Raw, &parsed)
	assert.NoError(t, err)

	for field, value := range parsed {
		if field == "apiVersion" {
			assert.Equal(t, "v2", value)
		}
	}
}

func TestStripDefaults(t *testing.T) {
	defaultConfig := DefaultClusterConfig()
	stripped := defaultConfig.StripDefaults()
	a := assert.New(t)
	a.Nil(stripped.Spec.API)
	a.Nil(stripped.Spec.ControllerManager)
	a.Nil(stripped.Spec.Scheduler)
	a.Nil(stripped.Spec.Network)
}

func TestDefaultClusterConfigYaml(t *testing.T) {
	data, err := yaml.Marshal(DefaultClusterConfig())
	assert.NoError(t, err)
	assert.NotContains(t, string(data), "status: {}")
}

func TestFeatureGates(t *testing.T) {
	yamlData := `
    apiVersion: k0s.k0sproject.io/v1beta1
    kind: ClusterConfig
    metadata:
      name: foobar
    spec:
      featureGates:
        - name: feature_XXX
          enabled: true
          components: ["x", "y", "z"]
        - name: feature_YYY
          enabled: true
        -
          name: feature_ZZZ
          enabled: false
`
	c, err := ConfigFromString(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(c.Spec.FeatureGates))
	assert.Equal(t, "feature_XXX", c.Spec.FeatureGates[0].Name)
	assert.True(t, c.Spec.FeatureGates[0].Enabled)
	for _, component := range []string{"x", "y", "z"} {
		value, found := c.Spec.FeatureGates[0].EnabledFor(component)
		assert.True(t, value)
		assert.True(t, found)
	}

	assert.Equal(t, "feature_YYY", c.Spec.FeatureGates[1].Name)
	assert.True(t, c.Spec.FeatureGates[1].Enabled)

	for _, k8sComponent := range KubernetesComponents {
		value, found := c.Spec.FeatureGates[1].EnabledFor(k8sComponent)
		assert.True(t, value)
		assert.True(t, found)
	}

	assert.Equal(t, "feature_ZZZ", c.Spec.FeatureGates[2].Name)

	assert.False(t, c.Spec.FeatureGates[2].Enabled)
}
