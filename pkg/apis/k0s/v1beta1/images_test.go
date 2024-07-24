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
	"fmt"
	"testing"

	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

func getConfigYAML(t *testing.T, c *ClusterConfig) []byte {
	res, err := yaml.Marshal(c)
	require.NoError(t, err)
	return res
}

func TestClusterImages_Customized(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1s
kind: ClusterConfig
spec:
  images:
    konnectivity:
      image: custom-repository/my-custom-konnectivity-image
      version: v0.0.1
    coredns:
      image: custom.io/coredns/coredns
      version: 1.0.0
`
	cfg, err := ConfigFromString(yamlData)
	require.NoError(t, err)
	a := cfg.Spec.Images

	assert.Equal(t, "custom-repository/my-custom-konnectivity-image:v0.0.1", a.Konnectivity.URI())
	assert.Equal(t, "1.0.0", a.CoreDNS.Version)
	assert.Equal(t, "custom.io/coredns/coredns", a.CoreDNS.Image)
	assert.Equal(t, "registry.k8s.io/metrics-server/metrics-server", a.MetricsServer.Image)
}

func TestImagesRepoOverrideInConfiguration(t *testing.T) {
	t.Run("if_has_repository_not_empty_add_prefix_to_all_images", func(t *testing.T) {
		t.Run("default_config", func(t *testing.T) {
			cfg := DefaultClusterConfig()
			cfg.Spec.Images.Repository = "my.repo"
			var testingConfig *ClusterConfig
			require.NoError(t, yaml.Unmarshal(getConfigYAML(t, cfg), &testingConfig))
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/apiserver-network-proxy-agent:%s", constant.KonnectivityImageVersion), testingConfig.Spec.Images.Konnectivity.URI())
			require.Equal(t, fmt.Sprintf("my.repo/metrics-server/metrics-server:%s", constant.MetricsImageVersion), testingConfig.Spec.Images.MetricsServer.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/kube-proxy:%s", constant.KubeProxyImageVersion), testingConfig.Spec.Images.KubeProxy.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/coredns:%s", constant.CoreDNSImageVersion), testingConfig.Spec.Images.CoreDNS.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-cni:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.CNI.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-node:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.Node.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-kube-controllers:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.KubeControllers.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-cni:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.CNI.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-node:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.Node.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-kube-controllers:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.KubeControllers.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/kube-router:%s", constant.KubeRouterCNIImageVersion), testingConfig.Spec.Images.KubeRouter.CNI.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/cni-node:%s", constant.KubeRouterCNIInstallerImageVersion), testingConfig.Spec.Images.KubeRouter.CNIInstaller.URI())
			require.Equal(t, fmt.Sprintf("my.repo/pause:%s", constant.KubePauseContainerImageVersion), testingConfig.Spec.Images.Pause.URI())
		})
		t.Run("config_with_custom_images", func(t *testing.T) {
			cfg := DefaultClusterConfig()
			cfg.Spec.Images.Konnectivity.Image = "my-custom-image"
			cfg.Spec.Images.Repository = "my.repo"
			var testingConfig *ClusterConfig
			require.NoError(t, yaml.Unmarshal(getConfigYAML(t, cfg), &testingConfig))
			require.Equal(t, fmt.Sprintf("my.repo/my-custom-image:%s", constant.KonnectivityImageVersion), testingConfig.Spec.Images.Konnectivity.URI())
			require.Equal(t, fmt.Sprintf("my.repo/metrics-server/metrics-server:%s", constant.MetricsImageVersion), testingConfig.Spec.Images.MetricsServer.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/kube-proxy:%s", constant.KubeProxyImageVersion), testingConfig.Spec.Images.KubeProxy.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/coredns:%s", constant.CoreDNSImageVersion), testingConfig.Spec.Images.CoreDNS.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-cni:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.CNI.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-node:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.Node.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k0sproject/calico-kube-controllers:%s", constant.CalicoComponentImagesVersion), testingConfig.Spec.Images.Calico.KubeControllers.URI())
		})
	})
}

func TestOverrideFunction(t *testing.T) {
	repository := "my.registry"
	testCases := []struct {
		Input  string
		Output string
	}{
		{
			Input:  "repo/image",
			Output: "my.registry/repo/image",
		},
		{
			Input:  "registry.com/repo/image",
			Output: "my.registry/repo/image",
		},
		{
			Input:  "image",
			Output: "my.registry/image",
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.Output, overrideRepository(repository, tc.Input))
	}
}

func TestImageSpec_Validate(t *testing.T) {
	validTestCases := []struct {
		Image   string
		Version string
	}{
		{"my.registry/repo/image", "v1.0.0"},
		{"my.registry/repo/image", "latest"},
		{"my.registry/repo/image", "v1.0.0-rc1"},
		{"my.registry/repo/image", "v1.0.0@sha256:0000000000000000000000000000000000000000000000000000000000000000"},
	}
	for _, tc := range validTestCases {
		t.Run(tc.Image+":"+tc.Version+"_valid", func(t *testing.T) {
			s := &ImageSpec{
				Image:   tc.Image,
				Version: tc.Version,
			}
			errs := s.Validate(field.NewPath("image"))
			assert.Empty(t, errs)
		})
	}

	errVersionRe := `must match regular expression: ^[\w][\w.-]{0,127}(?:@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,})?$`

	invalidTestCases := []struct {
		Image   string
		Version string
		Errs    field.ErrorList
	}{
		{
			"my.registry/repo/image", "",
			field.ErrorList{field.Invalid(field.NewPath("image").Child("version"), "", errVersionRe)},
		},
		// digest only is currently not supported
		{
			"my.registry/repo/image", "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			field.ErrorList{field.Invalid(field.NewPath("image").Child("version"), "sha256:0000000000000000000000000000000000000000000000000000000000000000", errVersionRe)},
		},
	}
	for _, tc := range invalidTestCases {
		t.Run(tc.Image+":"+tc.Version+"_valid", func(t *testing.T) {
			s := &ImageSpec{
				Image:   tc.Image,
				Version: tc.Version,
			}
			errs := s.Validate(field.NewPath("image"))
			assert.Equal(t, tc.Errs, errs)
		})
	}
}
