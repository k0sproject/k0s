package v1beta1

import (
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	"testing"
)

func getConfigYAML(t *testing.T, c *ClusterConfig) []byte {
	res, err := yaml.Marshal(c)
	require.NoError(t, err)
	return res
}

func TestImagesRepoOverride(t *testing.T) {
	t.Run("if_has_repository_not_empty_add_prefix_to_all_images", func(t *testing.T) {
		t.Run("default_config", func(t *testing.T) {
			cfg := DefaultClusterConfig()
			cfg.Images.Repository = "my.repo"
			var testingConfig *ClusterConfig
			require.NoError(t, yaml.Unmarshal(getConfigYAML(t, cfg), &testingConfig))

			require.Equal(t, "my.repo/us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent:v0.0.12", testingConfig.Images.Konnectivity.URI())
			require.Equal(t, "my.repo/gcr.io/k8s-staging-metrics-server/metrics-server:v0.3.7", testingConfig.Images.MetricsServer.URI())
			require.Equal(t, "my.repo/k8s.gcr.io/kube-proxy:v1.19.0", testingConfig.Images.KubeProxy.URI())
			require.Equal(t, "my.repo/docker.io/coredns/coredns:1.7.0", testingConfig.Images.CoreDNS.URI())
			require.Equal(t, "my.repo/calico/cni:v3.16.2", testingConfig.Images.Calico.CNI.URI())
			require.Equal(t, "my.repo/calico/pod2daemon-flexvol:v3.16.2", testingConfig.Images.Calico.FlexVolume.URI())
			require.Equal(t, "my.repo/calico/node:v3.16.2", testingConfig.Images.Calico.Node.URI())
			require.Equal(t, "my.repo/calico/kube-controllers:v3.16.2", testingConfig.Images.Calico.KubeControllers.URI())
		})
		t.Run("config_with_custom_images", func(t *testing.T) {
			cfg := DefaultClusterConfig()
			cfg.Images.Konnectivity.Image = "my-custom-image"
			cfg.Images.Repository = "my.repo"
			var testingConfig *ClusterConfig
			require.NoError(t, yaml.Unmarshal(getConfigYAML(t, cfg), &testingConfig))
			require.Equal(t, "my.repo/my-custom-image:v0.0.12", testingConfig.Images.Konnectivity.URI())
			require.Equal(t, "my.repo/gcr.io/k8s-staging-metrics-server/metrics-server:v0.3.7", testingConfig.Images.MetricsServer.URI())
			require.Equal(t, "my.repo/k8s.gcr.io/kube-proxy:v1.19.0", testingConfig.Images.KubeProxy.URI())
			require.Equal(t, "my.repo/docker.io/coredns/coredns:1.7.0", testingConfig.Images.CoreDNS.URI())
			require.Equal(t, "my.repo/calico/cni:v3.16.2", testingConfig.Images.Calico.CNI.URI())
			require.Equal(t, "my.repo/calico/pod2daemon-flexvol:v3.16.2", testingConfig.Images.Calico.FlexVolume.URI())
			require.Equal(t, "my.repo/calico/node:v3.16.2", testingConfig.Images.Calico.Node.URI())
			require.Equal(t, "my.repo/calico/kube-controllers:v3.16.2", testingConfig.Images.Calico.KubeControllers.URI())
		})
	})
}
