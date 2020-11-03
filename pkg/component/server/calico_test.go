package server

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type inMemorySaver map[string][]byte

func (i inMemorySaver) Save(dst string, content []byte) error {
	i[dst] = content
	return nil
}

func TestCalicoManifests(t *testing.T) {

	t.Run("must_write_crd_during_bootstrap", func(t *testing.T) {
		saver := inMemorySaver{}
		calico, err := NewCalico(v1beta1.DefaultClusterConfig(), saver)
		require.NoError(t, err)
		require.NoError(t, calico.Run())
		require.NoError(t, calico.Stop())

		for k := range saver {
			require.Contains(t, k, "calico-crd")
		}
	})

	t.Run("must_write_only_non_crd_on_change", func(t *testing.T) {
		saver := inMemorySaver{}
		calico, err := NewCalico(v1beta1.DefaultClusterConfig(), saver)
		require.NoError(t, err)

		_ = calico.processConfigChanges(calicoConfig{})

		for k := range saver {
			require.NotContains(t, k, "calico-crd")
		}
	})

	t.Run("must_have_wireguard_enabled_if_config_has", func(t *testing.T) {
		cfg := v1beta1.DefaultClusterConfig()
		cfg.Spec.Network.Calico.EnableWireguard = true
		saver := inMemorySaver{}
		calico, err := NewCalico(cfg, saver)
		require.NoError(t, err)

		_ = calico.processConfigChanges(calicoConfig{})

		daemonSetManifestRaw, foundRaw := saver["calico-DaemonSet-calico-node.yaml"]
		require.True(t, foundRaw, "must have daemon set for calico")
		spec := daemonSetContainersEnv{}
		require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
		found := false
		for _, container := range spec.Spec.Template.Spec.Containers {
			if container.Name != "calico-node" {
				continue
			}
			for _, envSpec := range container.Env {
				if envSpec.Name != "FELIX_WIREGUARDENABLED" {
					continue
				}
				found = true
				require.Equal(t, "true", envSpec.Value)
			}
		}
		require.True(t, found, "Must have FELIX_WIREGUARDENABLED env setting if config spec has wireguard enabled")
	})

	t.Run("must_not_have_wireguard_enabled_if_config_has_no", func(t *testing.T) {
		cfg := v1beta1.DefaultClusterConfig()
		cfg.Spec.Network.Calico.EnableWireguard = false
		saver := inMemorySaver{}
		calico, err := NewCalico(cfg, saver)
		require.NoError(t, err)

		_ = calico.processConfigChanges(calicoConfig{})

		daemonSetManifestRaw, foundRaw := saver["calico-DaemonSet-calico-node.yaml"]
		require.True(t, foundRaw, "must have daemon set for calico")
		spec := daemonSetContainersEnv{}
		require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
		found := false
		for _, container := range spec.Spec.Template.Spec.Containers {
			if container.Name != "calico-node" {
				continue
			}
			for _, envSpec := range container.Env {
				if envSpec.Name != "FELIX_WIREGUARDENABLED" {
					continue
				}
				found = true
			}
		}
		require.False(t, found, "Must not have FELIX_WIREGUARDENABLED env setting if config spec has no wireguard enabled")
	})
}

// this structure is needed only for unit tests and basocally it describes some fields that are needed to be parsed out of the daemon set manifest
type daemonSetContainersEnv struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name string `yaml:"name"`
					Env  []struct {
						Name      string      `yaml:"name"`
						Value     string      `yaml:"value"`
						ValueFrom interface{} `yaml:"valueFrom"`
					} `yaml:"env"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}
