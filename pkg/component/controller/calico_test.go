// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestCalicoManifests(t *testing.T) {
	newTestInstance := func(t *testing.T) *Calico {
		k0sVars, err := config.NewCfgVars(nil, t.TempDir())
		require.NoError(t, err)
		ctx := t.Context()
		calico := NewCalico(k0sVars)
		require.NoError(t, calico.Init(ctx))
		require.NoError(t, calico.Start(ctx))
		t.Cleanup(func() { assert.NoError(t, calico.Stop()) })
		return calico
	}

	clusterConfig := v1beta1.DefaultClusterConfig()
	clusterConfig.Spec.Network.Calico = v1beta1.DefaultCalico()
	clusterConfig.Spec.Network.Provider = "calico"
	clusterConfig.Spec.Network.KubeRouter = nil

	t.Run("must_write_only_non_crd_on_change", func(t *testing.T) {
		calico := newTestInstance(t)

		assert.NoError(t, calico.processConfigChanges(calicoConfig{}))

		if entries, err := os.ReadDir(filepath.Join(calico.k0sVars.ManifestsDir, "calico")); assert.NoError(t, err) {
			assert.NotEmpty(t, entries)
			for _, entry := range entries {
				assert.NotContains(t, entry.Name(), "calico-crd")
			}
		}
		if entries, err := os.ReadDir(filepath.Join(calico.k0sVars.ManifestsDir, "calico_init")); assert.NoError(t, err) {
			assert.Empty(t, entries)
		}
	})

	t.Run("must_have_wireguard_enabled_if_config_has", func(t *testing.T) {
		clusterConfig.Spec.Network.Calico.EnableWireguard = true
		calico := newTestInstance(t)
		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		require.NoError(t, calico.processConfigChanges(cfg))

		daemonSetManifestRaw, err := os.ReadFile(filepath.Join(calico.k0sVars.ManifestsDir, "calico", "calico-DaemonSet-calico-node.yaml"))
		require.NoError(t, err, "must have daemon set for calico")
		spec := daemonSetContainersEnv{}
		require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
		spec.RequireContainerHasEnvVariable(t, "calico-node", "FELIX_WIREGUARDENABLED", "true")
	})

	t.Run("must_not_have_wireguard_enabled_if_config_has_no", func(t *testing.T) {
		clusterConfig.Spec.Network.Calico.EnableWireguard = false
		calico := newTestInstance(t)

		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		_ = calico.processConfigChanges(cfg)

		daemonSetManifestRaw, err := os.ReadFile(filepath.Join(calico.k0sVars.ManifestsDir, "calico", "calico-DaemonSet-calico-node.yaml"))
		require.NoError(t, err, "must have daemon set for calico")
		spec := daemonSetContainersEnv{}
		require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
		spec.RequireContainerHasNoEnvVariable(t, "calico-node", "FELIX_WIREGUARDENABLED")
	})

	t.Run("ip_autodetection", func(t *testing.T) {
		t.Run("use_IPAutodetectionMethod_for_both_families_by_default", func(t *testing.T) {
			calicoNetSpec := clusterConfig.Spec.Network.Calico
			calicoNetSpec.IPAutodetectionMethod = "somemethod"
			calico := newTestInstance(t)
			templateContext, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			require.Equal(t, calicoNetSpec.IPAutodetectionMethod, templateContext.IPAutodetectionMethod)
			require.Equal(t, calicoNetSpec.IPAutodetectionMethod, templateContext.IPV6AutodetectionMethod,
				"IPv6 autodetection was not specified, hence it should be the same as the IPv4 autodetection method.")
			cfg, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			_ = calico.processConfigChanges(cfg)
			daemonSetManifestRaw, err := os.ReadFile(filepath.Join(calico.k0sVars.ManifestsDir, "calico", "calico-DaemonSet-calico-node.yaml"))
			require.NoError(t, err, "must have daemon set for calico")

			spec := daemonSetContainersEnv{}
			require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
			spec.RequireContainerHasEnvVariable(t, "calico-node", "IP6_AUTODETECTION_METHOD", templateContext.IPAutodetectionMethod)
			spec.RequireContainerHasEnvVariable(t, "calico-node", "IP_AUTODETECTION_METHOD", templateContext.IPAutodetectionMethod)
		})
		t.Run("use_IPV6AutodetectionMethod_for_ipv6_if_specified", func(t *testing.T) {
			clusterConfig.Spec.Network.Calico.IPAutodetectionMethod = "somemethod"
			clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod = "anothermethod"
			calico := newTestInstance(t)
			templateContext, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			require.Equal(t, clusterConfig.Spec.Network.Calico.IPAutodetectionMethod, templateContext.IPAutodetectionMethod)
			require.Equal(t, clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod, templateContext.IPV6AutodetectionMethod)
			cfg, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			_ = calico.processConfigChanges(cfg)
			daemonSetManifestRaw, err := os.ReadFile(filepath.Join(calico.k0sVars.ManifestsDir, "calico", "calico-DaemonSet-calico-node.yaml"))
			require.NoError(t, err, "must have daemon set for calico")

			spec := daemonSetContainersEnv{}
			require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
			spec.RequireContainerHasEnvVariable(t, "calico-node", "IP6_AUTODETECTION_METHOD", templateContext.IPV6AutodetectionMethod)
			spec.RequireContainerHasEnvVariable(t, "calico-node", "IP_AUTODETECTION_METHOD", templateContext.IPAutodetectionMethod)
		})
	})
}

// this structure is needed only for unit tests and basically it describes some fields that are needed to be parsed out of the daemon set manifest
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
				Volumes []struct {
					Name     string `yaml:"name"`
					HostPath struct {
						Type string `yaml:"type"`
						Path string `yaml:"path"`
					} `yaml:"hostPath"`
				} `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

func (ds daemonSetContainersEnv) RequireContainerHasEnvVariable(t *testing.T, containerName string, varName string, varValue string) {
	for _, container := range ds.Spec.Template.Spec.Containers {
		if container.Name != containerName {
			continue
		}
		found := false
		for _, envSpec := range container.Env {
			if envSpec.Name == varName {
				found = true
				require.Equal(t, envSpec.Value, varValue)
			}
		}
		require.Truef(t, found, "Variable %s not found", varName)
	}
}

func (ds daemonSetContainersEnv) RequireContainerHasNoEnvVariable(t *testing.T, containerName string, varName string) {
	for _, container := range ds.Spec.Template.Spec.Containers {
		if container.Name != containerName {
			continue
		}
		found := false
		for _, envSpec := range container.Env {
			if envSpec.Name == varName {
				found = true
			}
		}
		require.Falsef(t, found, "Variable %s must not be found", varName)
	}
}
