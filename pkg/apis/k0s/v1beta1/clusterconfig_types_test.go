// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/iface"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterDefaults(t *testing.T) {
	c, err := ConfigFromBytes([]byte("apiVersion: k0s.k0sproject.io/v1beta1"))
	assert.NoError(t, err)
	assert.Equal(t, DefaultStorageSpec(), c.Spec.Storage)
}

func TestUnknownFieldValidation(t *testing.T) {
	_, err := ConfigFromBytes([]byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
unknown: 1`))

	assert.Error(t, err)
}

func TestStorageDefaults(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
`)

	c, err := ConfigFromBytes(yamlData)
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
	validTestCase.Spec.Images.Konnectivity = &ImageSpec{
		Image:   "foo",
		Version: "v1",
	}
	validTestCase.Spec.Images.PushGateway = &ImageSpec{
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
	invalidTestCase.Spec.Images.MetricsServer = &ImageSpec{
		Image: "baz",
		// digest only is currently not supported
		Version: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	invalidTestCase.Spec.Images.Calico.CNI = &ImageSpec{
		Image: "qux",
		// digest only is currently not supported
		Version: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	invalidTestCase.Spec.Images.KubeRouter.CNI = &ImageSpec{
		Image: "quux",
		// digest only is currently not supported
		Version: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}

	errs = invalidTestCase.Validate()
	assert.Len(t, errs, 3)
}

func TestEtcdDefaults(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  storage:
    type: etcd
`)

	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, EtcdStorageType, c.Spec.Storage.Type)
	addr, err := iface.FirstPublicAddress()
	assert.NoError(t, err)
	assert.Equal(t, addr, c.Spec.Storage.Etcd.PeerAddress)
}

func TestNetworkValidation_Custom(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: custom
  storage:
    type: etcd
`)

	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Zero(t, errors)
}

func TestNetworkValidation_Calico(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: calico
  storage:
    type: etcd
`)

	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Zero(t, errors)
}

func TestNetworkValidation_Invalid(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  network:
    provider: invalidProvider
  storage:
    type: etcd
`)

	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	if assert.Len(t, errors, 1) {
		assert.ErrorContains(t, errors[0], `spec: network: provider: Unsupported value: "invalidProvider": supported values: "kuberouter", "calico", "custom"`)
	}
}

func TestApiExternalAddress(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  api:
    externalAddress: foo.bar.com
    address: 1.2.3.4
`)

	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "https://foo.bar.com:6443", c.Spec.API.APIAddressURL())
	assert.Equal(t, "https://foo.bar.com:9443", c.Spec.API.K0sControlPlaneAPIAddress())
}

func TestApiNoExternalAddress(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  api:
    address: 1.2.3.4
`)

	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "https://1.2.3.4:6443", c.Spec.API.APIAddressURL())
	assert.Equal(t, "https://1.2.3.4:9443", c.Spec.API.K0sControlPlaneAPIAddress())
}

func TestNullValues(t *testing.T) {
	yamlData := []byte(`
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
`)
	extensionsYamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  extensions:
    storage: null
`)

	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	assert.False(t, c.Spec.Telemetry.IsEnabled())
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

	e, err := ConfigFromBytes(extensionsYamlData)
	assert.NoError(t, err)
	assert.Equal(t, DefaultExtensions(), e.Spec.Extensions)
}

func TestWorkerProfileConfig(t *testing.T) {
	yamlData := []byte(`
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
`)
	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	require.Len(t, c.Spec.WorkerProfiles, 2)
	assert.Equal(t, "profile_XXX", c.Spec.WorkerProfiles[0].Name)
	assert.Equal(t, "profile_YYY", c.Spec.WorkerProfiles[1].Name)

	j := c.Spec.WorkerProfiles[1].Config
	var parsed map[string]any

	err = json.Unmarshal(j.Raw, &parsed)
	assert.NoError(t, err)

	for field, value := range parsed {
		if field == "apiVersion" {
			assert.Equal(t, "v2", value)
		}
	}
}

func TestClusterConfig_StripDefaults_Zero(t *testing.T) {
	underTest := ClusterConfig{}
	assert.Equal(t, &underTest, underTest.StripDefaults())
}

func TestClusterConfig_StripDefaults_ZeroSpec(t *testing.T) {
	underTest := ClusterConfig{Spec: &ClusterSpec{}}
	assert.Equal(t, &underTest, underTest.StripDefaults())
}

func TestClusterConfig_StripDefaults_DefaultConfig(t *testing.T) {
	defaultConfig := DefaultClusterConfig()
	stripped := defaultConfig.StripDefaults()
	a := assert.New(t)
	a.Nil(stripped.Spec.API)
	a.Nil(stripped.Spec.ControllerManager)
	a.Nil(stripped.Spec.Scheduler)
	a.Nil(stripped.Spec.Storage)
	a.Nil(stripped.Spec.Network)
	a.Nil(stripped.Spec.Telemetry)
	a.Nil(stripped.Spec.Images)
	a.Nil(stripped.Spec.Konnectivity)
}

func TestClusterConfig_GetClusterWideConfig(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    *ClusterConfig
		expected *ClusterConfig
	}{
		{"nil", nil, nil},
		{"zero", &ClusterConfig{}, &ClusterConfig{}},
		{"zero spec", &ClusterConfig{Spec: &ClusterSpec{}}, &ClusterConfig{Spec: &ClusterSpec{}}},
		{"zero network", &ClusterConfig{Spec: &ClusterSpec{
			Network: &Network{},
		}}, &ClusterConfig{Spec: &ClusterSpec{
			Network: &Network{},
		}}},
		{"RemovesNodeConfig", &ClusterConfig{Spec: &ClusterSpec{
			API: &APISpec{Address: "127.0.0.1"},
			Storage: &StorageSpec{
				Type: KineStorageType,
			},
			Install: &InstallSpec{
				SystemUsers: &SystemUser{
					KubeAPIServer: t.Name(),
				},
			},
			Network: &Network{
				Provider:      "calico",
				ServiceCIDR:   "ServiceCIDR",
				ClusterDomain: "ClusterDomain",
				ControlPlaneLoadBalancing: &ControlPlaneLoadBalancingSpec{
					Enabled: true,
				},
				PrimaryAddressFamily: PrimaryFamilyIPv6,
			}},
		}, &ClusterConfig{Spec: &ClusterSpec{
			Network: &Network{
				Provider: "calico",
			},
		}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input.GetClusterWideConfig())
		})
	}
}

func TestClusterConfig_StripDefaults_Images(t *testing.T) {
	//nolint:dupword // it's YAML data
	yaml := `
spec:
   images:
     calico:
       cni:
         image: registry.acme.corp/k0sproject/cni
       kubecontrollers:
         image: registry.acme.corp/k0sproject/kubecontrollers
       node:
         image: registry.acme.corp/k0sproject/node
     coredns:
       image: registry.acme.corp/k0sproject/coredns
     konnectivity:
       image: registry.acme.corp/k0sproject/konnectivity
     kubeproxy:
       image: registry.acme.corp/k0sproject/kubeproxy
     kuberouter:
       cni:
         image: registry.acme.corp/k0sproject/cni
       cniInstaller:
         image: registry.acme.corp/k0sproject/cniinstaller
     metricsserver:
       image: registry.acme.corp/k0sproject/metricsserver
     pause:
       image: registry.acme.corp/k0sproject/pause
     pushgateway:
       image: registry.acme.corp/k0sproject/pushgateway
   network:
     nodeLocalLoadBalancing:
       %s
       envoyProxy:
         image:
           image: registry.acme.corp/k0sproject/envoy
       traefik:
         image:
           image: registry.acme.corp/k0sproject/traefik
`
	for _, tt := range []struct {
		name    string
		nllb    NllbType
		snippet string
	}{
		{"default", NllbTypeEnvoyProxy, ""},
		{"envoy", NllbTypeEnvoyProxy, "type: EnvoyProxy"},
		{"traefik", NllbTypeTraefik, "type: Traefik"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input, err := ConfigFromBytes(fmt.Appendf(nil, yaml, tt.snippet))
			require.NoError(t, err)

			stripped := input.StripDefaults()

			nllb := stripped.Spec.Network.NodeLocalLoadBalancing

			switch tt.nllb {
			case NllbTypeEnvoyProxy:
				if assert.NotNil(t, nllb.EnvoyProxy) {
					assert.Equal(t, "registry.acme.corp/k0sproject/envoy", stripped.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image)
					assert.NotEmpty(t, stripped.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Version)
				}
				assert.Nil(t, nllb.Traefik)
			case NllbTypeTraefik:
				if assert.NotNil(t, nllb.Traefik) {
					assert.Equal(t, "registry.acme.corp/k0sproject/traefik", nllb.Traefik.Image.Image)
					assert.NotEmpty(t, nllb.Traefik.Image.Version)
				}
				assert.Nil(t, nllb.EnvoyProxy)
			}

			assert.Equal(t, "registry.acme.corp/k0sproject/konnectivity", stripped.Spec.Images.Konnectivity.Image)
			assert.NotEmpty(t, stripped.Spec.Images.Konnectivity.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/pushgateway", stripped.Spec.Images.PushGateway.Image)
			assert.NotEmpty(t, stripped.Spec.Images.PushGateway.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/metricsserver", stripped.Spec.Images.MetricsServer.Image)
			assert.NotEmpty(t, stripped.Spec.Images.MetricsServer.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/kubeproxy", stripped.Spec.Images.KubeProxy.Image)
			assert.NotEmpty(t, stripped.Spec.Images.KubeProxy.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/coredns", stripped.Spec.Images.CoreDNS.Image)
			assert.NotEmpty(t, stripped.Spec.Images.CoreDNS.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/pause", stripped.Spec.Images.Pause.Image)
			assert.NotEmpty(t, stripped.Spec.Images.Pause.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/cni", stripped.Spec.Images.Calico.CNI.Image)
			assert.NotEmpty(t, stripped.Spec.Images.Calico.CNI.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/node", stripped.Spec.Images.Calico.Node.Image)
			assert.NotEmpty(t, stripped.Spec.Images.Calico.Node.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/kubecontrollers", stripped.Spec.Images.Calico.KubeControllers.Image)
			assert.NotEmpty(t, stripped.Spec.Images.Calico.KubeControllers.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/cni", stripped.Spec.Images.KubeRouter.CNI.Image)
			assert.NotEmpty(t, stripped.Spec.Images.KubeRouter.CNI.Version)
			assert.Equal(t, "registry.acme.corp/k0sproject/cniinstaller", stripped.Spec.Images.KubeRouter.CNIInstaller.Image)
			assert.NotEmpty(t, stripped.Spec.Images.KubeRouter.CNIInstaller.Version)
		})
	}
}

func TestStrippedClusterWideDefaultConfig(t *testing.T) {
	defaultConfig := DefaultClusterConfig()
	stripped := defaultConfig.GetClusterWideConfig().StripDefaults()
	if assert.NotNil(t, stripped.Spec) {
		// The network and extensions fields aren't properly handled at the moment.
		stripped.Spec.Network = nil
		stripped.Spec.Extensions = nil
		assert.Zero(t, *stripped.Spec, "%+v", stripped.Spec)
	}
}

func TestDefaultClusterConfigYaml(t *testing.T) {
	data, err := yaml.Marshal(DefaultClusterConfig())
	assert.NoError(t, err)
	assert.NotContains(t, string(data), "status: {}")
}

func TestFeatureGates(t *testing.T) {
	yamlData := []byte(`
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
`)
	c, err := ConfigFromBytes(yamlData)
	assert.NoError(t, err)
	require.Len(t, c.Spec.FeatureGates, 3)
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
