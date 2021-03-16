package controller

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/stretchr/testify/require"
)

func TestKubeRouterManifests(t *testing.T) {
	t.Run("must_defaults", func(t *testing.T) {
		cfg := v1beta1.DefaultClusterConfig()
		cfg.Spec.Network.Calico = nil
		cfg.Spec.Network.Provider = "kuberouter"
		saver := inMemorySaver{}
		kr, err := NewKubeRouter(cfg, saver)
		require.NoError(t, err)
		require.NoError(t, kr.Run())
		require.NoError(t, kr.Stop())

		_, foundRaw := saver["kube-router.yaml"]
		require.True(t, foundRaw, "must have daemon set for kube-router")
		// spec := daemonSetContainersEnv{}
		// require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
		// found := false
		// for _, container := range spec.Spec.Template.Spec.Containers {
		// 	if container.Name != "calico-node" {
		// 		continue
		// 	}
		// 	for _, envSpec := range container.Env {
		// 		if envSpec.Name != "FELIX_WIREGUARDENABLED" {
		// 			continue
		// 		}
		// 		found = true
		// 		require.Equal(t, "true", envSpec.Value)
		// 	}
		// }
		// require.True(t, found, "Must have FELIX_WIREGUARDENABLED env setting if config spec has wireguard enabled")
	})

}
