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
	"context"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

type inMemorySaver map[string][]byte

func (i inMemorySaver) Save(dst string, content []byte) error {
	i[dst] = content
	return nil
}

func TestCalicoManifests(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	clusterConfig := v1beta1.DefaultClusterConfig()
	clusterConfig.Spec.Network.Calico = v1beta1.DefaultCalico()
	clusterConfig.Spec.Network.Provider = "calico"
	clusterConfig.Spec.Network.KubeRouter = nil

	t.Run("must_write_crd_during_bootstrap", func(t *testing.T) {
		saver := inMemorySaver{}
		crdSaver := inMemorySaver{}
		calico := NewCalico(k0sVars, crdSaver, saver)
		require.NoError(t, calico.Start(context.Background()))
		require.NoError(t, calico.Stop())

		for k := range crdSaver {
			require.Contains(t, k, "calico-crd")
		}
		require.Len(t, saver, 0)
	})

	t.Run("must_write_only_non_crd_on_change", func(t *testing.T) {
		saver := inMemorySaver{}
		crdSaver := inMemorySaver{}
		calico := NewCalico(k0sVars, crdSaver, saver)

		_ = calico.processConfigChanges(calicoConfig{})

		for k := range saver {
			require.NotContains(t, k, "calico-crd")
		}
		require.Len(t, crdSaver, 0)
	})

	t.Run("must_have_wireguard_enabled_if_config_has", func(t *testing.T) {
		clusterConfig.Spec.Network.Calico.EnableWireguard = true
		saver := inMemorySaver{}
		crdSaver := inMemorySaver{}
		calico := NewCalico(k0sVars, crdSaver, saver)
		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		_ = calico.processConfigChanges(cfg)

		daemonSetManifestRaw, foundRaw := saver["calico-DaemonSet-calico-node.yaml"]
		require.True(t, foundRaw, "must have daemon set for calico")
		spec := daemonSetContainersEnv{}
		require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
		spec.RequireContainerHasEnvVariable(t, "calico-node", "FELIX_WIREGUARDENABLED", "true")
	})

	t.Run("must_not_have_wireguard_enabled_if_config_has_no", func(t *testing.T) {
		clusterConfig.Spec.Network.Calico.EnableWireguard = false
		saver := inMemorySaver{}
		crdSaver := inMemorySaver{}
		calico := NewCalico(k0sVars, crdSaver, saver)

		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		_ = calico.processConfigChanges(cfg)

		daemonSetManifestRaw, foundRaw := saver["calico-DaemonSet-calico-node.yaml"]
		require.True(t, foundRaw, "must have daemon set for calico")
		spec := daemonSetContainersEnv{}
		require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
		spec.RequireContainerHasNoEnvVariable(t, "calico-node", "FELIX_WIREGUARDENABLED")
	})

	t.Run("ip_autodetection", func(t *testing.T) {
		t.Run("use_IPAutodetectionMethod_for_both_families_by_default", func(t *testing.T) {
			clusterConfig.Spec.Network.Calico.IPAutodetectionMethod = "somemethod"
			saver := inMemorySaver{}
			crdSaver := inMemorySaver{}
			calico := NewCalico(k0sVars, crdSaver, saver)
			templateContext, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			require.Equal(t, clusterConfig.Spec.Network.Calico.IPAutodetectionMethod, templateContext.IPAutodetectionMethod)
			require.Equal(t, templateContext.IPV6AutodetectionMethod, templateContext.IPV6AutodetectionMethod)
			cfg, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			_ = calico.processConfigChanges(cfg)
			daemonSetManifestRaw, foundRaw := saver["calico-DaemonSet-calico-node.yaml"]
			require.True(t, foundRaw, "must have daemon set for calico")

			spec := daemonSetContainersEnv{}
			require.NoError(t, yaml.Unmarshal(daemonSetManifestRaw, &spec))
			spec.RequireContainerHasEnvVariable(t, "calico-node", "IP6_AUTODETECTION_METHOD", templateContext.IPAutodetectionMethod)
			spec.RequireContainerHasEnvVariable(t, "calico-node", "IP_AUTODETECTION_METHOD", templateContext.IPAutodetectionMethod)
		})
		t.Run("use_IPV6AutodetectionMethod_for_ipv6_if_specified", func(t *testing.T) {
			clusterConfig.Spec.Network.Calico.IPAutodetectionMethod = "somemethod"
			clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod = "anothermethod"
			saver := inMemorySaver{}
			crdSaver := inMemorySaver{}
			calico := NewCalico(k0sVars, crdSaver, saver)
			templateContext, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			require.Equal(t, clusterConfig.Spec.Network.Calico.IPAutodetectionMethod, templateContext.IPAutodetectionMethod)
			require.Equal(t, clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod, templateContext.IPV6AutodetectionMethod)
			cfg, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			_ = calico.processConfigChanges(cfg)
			daemonSetManifestRaw, foundRaw := saver["calico-DaemonSet-calico-node.yaml"]

			require.True(t, foundRaw, "must have daemon set for calico")
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
		require.True(t, found, "Variable %s not found", varName)
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
		require.False(t, found, "Variable %s must not be found", varName)
	}
}
