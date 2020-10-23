/*
Copyright 2020 Mirantis, Inc.

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

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func getConfigYAML(t *testing.T, c *ClusterConfig) []byte {
	res, err := yaml.Marshal(c)
	require.NoError(t, err)
	return res
}

func TestImagesRepoOverrideInConfiguration(t *testing.T) {
	t.Run("if_has_repository_not_empty_add_prefix_to_all_images", func(t *testing.T) {
		t.Run("default_config", func(t *testing.T) {
			cfg := DefaultClusterConfig()
			cfg.Images.Repository = "my.repo"
			var testingConfig *ClusterConfig
			require.NoError(t, yaml.Unmarshal(getConfigYAML(t, cfg), &testingConfig))
			require.Equal(t, fmt.Sprintf("my.repo/k8s-artifacts-prod/kas-network-proxy/proxy-agent:%s", constant.KonnectivityImageVersion), testingConfig.Images.Konnectivity.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k8s-staging-metrics-server/metrics-server:%s", constant.MetricsImageVersion), testingConfig.Images.MetricsServer.URI())
			require.Equal(t, fmt.Sprintf("my.repo/kube-proxy:%s", constant.KubeProxyImageVersion), testingConfig.Images.KubeProxy.URI())
			require.Equal(t, fmt.Sprintf("my.repo/coredns/coredns:%s", constant.CoreDNSImageVersion), testingConfig.Images.CoreDNS.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/cni:%s", constant.CalicoImageVersion), testingConfig.Images.Calico.CNI.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/pod2daemon-flexvol:%s", constant.FlexVolumeImageVersion), testingConfig.Images.Calico.FlexVolume.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/node:%s", constant.CalicoNodeImageVersion), testingConfig.Images.Calico.Node.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/kube-controllers:%s", constant.KubeControllerImageVersion), testingConfig.Images.Calico.KubeControllers.URI())
		})
		t.Run("config_with_custom_images", func(t *testing.T) {
			cfg := DefaultClusterConfig()
			cfg.Images.Konnectivity.Image = "my-custom-image"
			cfg.Images.Repository = "my.repo"
			var testingConfig *ClusterConfig
			require.NoError(t, yaml.Unmarshal(getConfigYAML(t, cfg), &testingConfig))
			require.Equal(t, fmt.Sprintf("my.repo/my-custom-image:%s", constant.KonnectivityImageVersion), testingConfig.Images.Konnectivity.URI())
			require.Equal(t, fmt.Sprintf("my.repo/k8s-staging-metrics-server/metrics-server:%s", constant.MetricsImageVersion), testingConfig.Images.MetricsServer.URI())
			require.Equal(t, fmt.Sprintf("my.repo/kube-proxy:%s", constant.KubeProxyImageVersion), testingConfig.Images.KubeProxy.URI())
			require.Equal(t, fmt.Sprintf("my.repo/coredns/coredns:%s", constant.CoreDNSImageVersion), testingConfig.Images.CoreDNS.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/cni:%s", constant.CalicoImageVersion), testingConfig.Images.Calico.CNI.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/pod2daemon-flexvol:%s", constant.FlexVolumeImageVersion), testingConfig.Images.Calico.FlexVolume.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/node:%s", constant.CalicoNodeImageVersion), testingConfig.Images.Calico.Node.URI())
			require.Equal(t, fmt.Sprintf("my.repo/calico/kube-controllers:%s", constant.KubeControllerImageVersion), testingConfig.Images.Calico.KubeControllers.URI())
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
