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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalicoManifests(t *testing.T) {
	newTestInstance := func(t *testing.T) (*Calico, *test.Hook) {
		log, logs := test.NewNullLogger()
		manifestsDir := t.TempDir()
		ctx := t.Context()
		calico, err := NewCalico(v1beta1.DefaultClusterConfig(), manifestsDir, func() (*bool, <-chan struct{}) { return new(true), nil })
		require.NoError(t, err)
		calico.log = log
		require.NoError(t, calico.Init(ctx))
		require.NoError(t, calico.Start(ctx))
		t.Cleanup(func() { assert.NoError(t, calico.Stop()) })
		return calico, logs
	}

	assertNoLogEntries := func(t *testing.T, logs *test.Hook) {
		t.Helper()
		if entries := logs.AllEntries(); !assert.Empty(t, entries, "Unexpected log output") {
			for _, entry := range entries {
				t.Log(entry.String())
			}
		}
	}

	clusterConfig := v1beta1.DefaultClusterConfig()
	clusterConfig.Spec.Network.Calico = v1beta1.DefaultCalico()
	clusterConfig.Spec.Network.Provider = "calico"
	clusterConfig.Spec.Network.KubeRouter = nil

	t.Run("must_write_only_non_crd_on_change", func(t *testing.T) {
		calico, logs := newTestInstance(t)

		assert.NoError(t, calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, &calicoClusterConfig{}, false}, nil))

		assertNoLogEntries(t, logs)
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
		calico, logs := newTestInstance(t)
		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		require.NoError(t, calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil))

		assertNoLogEntries(t, logs)
		daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
		calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
		require.NotNil(t, calicoNode, "No calico-node container")
		assertEnvVar(t, calicoNode.Env, "FELIX_WIREGUARDENABLED", new("true"))
	})

	t.Run("must_not_have_wireguard_enabled_if_config_has_no", func(t *testing.T) {
		clusterConfig.Spec.Network.Calico.EnableWireguard = false
		calico, logs := newTestInstance(t)

		cfg, err := calico.getConfig(clusterConfig)
		require.NoError(t, err)
		err = calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil)
		require.NoError(t, err)

		assertNoLogEntries(t, logs)
		daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
		calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
		require.NotNil(t, calicoNode, "No calico-node container")
		assertEnvVar(t, calicoNode.Env, "FELIX_WIREGUARDENABLED", nil)
	})

	t.Run("ip_autodetection", func(t *testing.T) {
		t.Run("use_IPAutodetectionMethod_for_both_families_by_default", func(t *testing.T) {
			calicoNetSpec := clusterConfig.Spec.Network.Calico
			calicoNetSpec.IPAutodetectionMethod = "somemethod"
			calico, logs := newTestInstance(t)
			templateContext, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			require.Equal(t, calicoNetSpec.IPAutodetectionMethod, templateContext.IPAutodetectionMethod)
			require.Equal(t, calicoNetSpec.IPAutodetectionMethod, templateContext.IPV6AutodetectionMethod,
				"IPv6 autodetection was not specified, hence it should be the same as the IPv4 autodetection method.")
			cfg, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			err = calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil)
			require.NoError(t, err)

			assertNoLogEntries(t, logs)
			daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
			calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
			require.NotNil(t, calicoNode, "No calico-node container")
			assertEnvVar(t, calicoNode.Env, "IP6_AUTODETECTION_METHOD", &templateContext.IPAutodetectionMethod)
			assertEnvVar(t, calicoNode.Env, "IP_AUTODETECTION_METHOD", &templateContext.IPAutodetectionMethod)
		})
		t.Run("use_IPV6AutodetectionMethod_for_ipv6_if_specified", func(t *testing.T) {
			clusterConfig.Spec.Network.Calico.IPAutodetectionMethod = "somemethod"
			clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod = "anothermethod"
			calico, logs := newTestInstance(t)
			templateContext, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			require.Equal(t, clusterConfig.Spec.Network.Calico.IPAutodetectionMethod, templateContext.IPAutodetectionMethod)
			require.Equal(t, clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod, templateContext.IPV6AutodetectionMethod)
			cfg, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			err = calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil)
			require.NoError(t, err)

			assertNoLogEntries(t, logs)
			daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node.yaml")
			calicoNode := findContainer(daemonSet.Spec.Template.Spec.Containers, "calico-node")
			require.NotNil(t, calicoNode, "No calico-node container")
			assertEnvVar(t, calicoNode.Env, "IP6_AUTODETECTION_METHOD", &templateContext.IPV6AutodetectionMethod)
			assertEnvVar(t, calicoNode.Env, "IP_AUTODETECTION_METHOD", &templateContext.IPAutodetectionMethod)
		})
	})

	t.Run("windows_manifests", func(t *testing.T) {
		newClusterConfig := func() *v1beta1.ClusterConfig {
			clusterConfig := v1beta1.DefaultClusterConfig()
			clusterConfig.Spec.Network.Calico = v1beta1.DefaultCalico()
			clusterConfig.Spec.Network.Provider = "calico"
			clusterConfig.Spec.Network.KubeRouter = nil
			return clusterConfig
		}

		t.Run("rendered_for_vxlan_clusters_with_windows_nodes", func(t *testing.T) {
			clusterConfig := newClusterConfig()
			cniImage := clusterConfig.Spec.Images.Calico.Windows.CNI.URI()
			nodeImage := clusterConfig.Spec.Images.Calico.Windows.Node.URI()

			calico, logs := newTestInstance(t)
			cfg, err := calico.getConfig(clusterConfig)
			require.NoError(t, err)
			require.NoError(t, calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, true}, nil))
			assertNoLogEntries(t, logs)

			daemonSet := requireResource[appsv1.DaemonSet](t, calico.manifestsDir, "calico", "calico-DaemonSet-calico-node-windows.yaml")

			for _, c := range []struct{ name, image string }{
				{"uninstall-calico", nodeImage}, {"install-cni", cniImage},
			} {
				found := findContainer(daemonSet.Spec.Template.Spec.InitContainers, c.name)
				if assert.NotNilf(t, found, "No such initContainer: %s", c.name) {
					assert.Equalf(t, c.image, found.Image, "Unexpected image for initContainer %s", c.name)
				}
			}
			for _, name := range []string{"felix", "node"} {
				c := findContainer(daemonSet.Spec.Template.Spec.Containers, name)
				if assert.NotNilf(t, c, "No such container: %s", name) {
					assert.Equalf(t, nodeImage, c.Image, "Unexpected image for container %s", name)
				}
			}

			if c := findContainer(daemonSet.Spec.Template.Spec.InitContainers, "install-cni"); assert.NotNilf(t, c, "No install-cni init container") {
				assertEnvVar(t, c.Env, "KUBERNETES_DNS_SERVERS", new("10.96.0.10"))
				assertEnvVar(t, c.Env, "KUBERNETES_SERVICE_CIDRS", new("10.96.0.0/12"))
			}

			if c := findContainer(daemonSet.Spec.Template.Spec.Containers, "felix"); assert.NotNilf(t, c, "No felix container") {
				assertEnvVar(t, c.Env, "FELIX_VXLANPORT", new("4789"))
				assertEnvVar(t, c.Env, "FELIX_VXLANVNI", new("4096"))
			}

			ipamConfig := requireResource[unstructured.Unstructured](t, calico.manifestsDir, "calico", "calico-IPAMConfig-default.yaml")
			assert.Equal(t, "IPAMConfig", ipamConfig.GetKind())
		})

		for _, tt := range []struct {
			name           string
			mode           v1beta1.CalicoMode
			includeWindows bool
		}{
			{name: "not_rendered_without_windows_nodes"},
			{name: "not_rendered_in_bird_mode", mode: v1beta1.CalicoModeBIRD, includeWindows: true},
		} {
			t.Run(tt.name, func(t *testing.T) {
				clusterConfig := newClusterConfig()
				if tt.mode != "" {
					clusterConfig.Spec.Network.Calico.Mode = tt.mode
				}
				calico, logs := newTestInstance(t)
				cfg, err := calico.getConfig(clusterConfig)
				require.NoError(t, err)
				require.NoError(t, calico.processConfigChanges(&calicoConfig{&calico.nodeConfig, cfg, tt.includeWindows}, nil))
				assertNoLogEntries(t, logs)

				for _, name := range []string{"DaemonSet-calico-node-windows", "IPAMConfig-default"} {
					resources, err := readResources(filepath.Join(calico.manifestsDir, "calico", "calico-"+name+".yaml"))
					if assert.NoError(t, err) {
						assert.Emptyf(t, resources, "Expected no resources for %s", name)
					}
				}
			})
		}
	})
}

func readResources(path string) ([]*unstructured.Unstructured, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return testutil.ParseManifests(raw)
}

func requireResource[T any](t *testing.T, path ...string) *T {
	t.Helper()

	name := filepath.Join(path...)
	resources, err := readResources(name)
	require.NoErrorf(t, err, "While trying to read manifests from %s", name)

	var resource T
	require.Lenf(t, resources, 1, "Expected a single %T in %s", &resource, name)
	err = scheme.Scheme.Convert(resources[0], &resource, nil)
	require.NoErrorf(t, err, "While parsing %s into %T", name, &resource)
	return &resource
}

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
