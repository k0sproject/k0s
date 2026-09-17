// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalicoManifests(t *testing.T) {
	newTestInstance := func(t *testing.T) *Calico {
		manifestsDir := t.TempDir()
		ctx := t.Context()
		calico, err := NewCalico(v1beta1.DefaultClusterConfig(), manifestsDir, func() (*bool, <-chan struct{}) { return new(true), nil })
		require.NoError(t, err)
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

		assert.NoError(t, calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, &calicoClusterConfig{}, false}, nil))

		if entries, err := os.ReadDir(filepath.Join(calico.manifestsDir, "calico")); assert.NoError(t, err) {
			assert.NotEmpty(t, entries)
			for _, entry := range entries {
				assert.NotContains(t, entry.Name(), "calico-crd")
			}
		}
		if entries, err := os.ReadDir(filepath.Join(calico.manifestsDir, "calico_init")); assert.NoError(t, err) {
			assert.Empty(t, entries)
		}
	})

	t.Run("must_have_wireguard_enabled_if_config_has", func(t *testing.T) {
		clusterConfig.Spec.Network.Calico.EnableWireguard = true
		calico := newTestInstance(t)
		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		require.NoError(t, calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil))

		daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
		calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
		require.NotNil(t, calicoNode, "No calico-node container")
		assertEnvVar(t, calicoNode.Env, "FELIX_WIREGUARDENABLED", new("true"))
	})

	t.Run("must_not_have_wireguard_enabled_if_config_has_no", func(t *testing.T) {
		clusterConfig.Spec.Network.Calico.EnableWireguard = false
		calico := newTestInstance(t)

		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		err = calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil)
		require.NoError(t, err)

		daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
		calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
		require.NotNil(t, calicoNode, "No calico-node container")
		assertEnvVar(t, calicoNode.Env, "FELIX_WIREGUARDENABLED", nil)
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
			err = calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil)
			require.NoError(t, err)

			daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
			calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
			require.NotNil(t, calicoNode, "No calico-node container")
			assertEnvVar(t, calicoNode.Env, "IP6_AUTODETECTION_METHOD", &templateContext.IPAutodetectionMethod)
			assertEnvVar(t, calicoNode.Env, "IP_AUTODETECTION_METHOD", &templateContext.IPAutodetectionMethod)
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
			err = calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil)
			require.NoError(t, err)

			daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
			calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
			require.NotNil(t, calicoNode, "No calico-node container")
			assertEnvVar(t, calicoNode.Env, "IP6_AUTODETECTION_METHOD", &templateContext.IPV6AutodetectionMethod)
			assertEnvVar(t, calicoNode.Env, "IP_AUTODETECTION_METHOD", &templateContext.IPAutodetectionMethod)
		})
	})
}

func requireResource[T any](t *testing.T, path ...string) *T {
	t.Helper()

	name := filepath.Join(path...)
	raw, err := os.ReadFile(name)
	require.NoError(t, err)
	resources, err := testutil.ParseManifests(raw)
	require.NoErrorf(t, err, "While parsing %s", name)

	var resource T
	require.Lenf(t, resources, 1, "Expected a single %T in %s", &resource, name)
	err = scheme.Scheme.Convert(resources[0], &resource, nil)
	require.NoErrorf(t, err, "While parsing %s into %T", name, &resource)
	return &resource
}

//nolint:unparam // general-purpose helper
func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

func assertEnvVar(t *testing.T, vars []corev1.EnvVar, name string, value *string) {
	t.Helper()
	for _, env := range vars {
		if env.Name == name {
			if value == nil {
				assert.Failf(t, "Expected environment variable to be unset", "Actual value of %s: %q", name, env.Value)
			} else {
				assert.Equalf(t, *value, env.Value, "Unexpected value of environment variable %s", name)
			}
			return
		}
	}
	if value != nil {
		assert.Failf(t, "Expected environment variable to be present", "Expected value of %s: %q", name, *value)
	}
}
