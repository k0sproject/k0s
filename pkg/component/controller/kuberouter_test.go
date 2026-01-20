// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func TestKubeRouterManifests(t *testing.T) {
	tests := []struct {
		name            string
		setupNodeCfg    func(*v1beta1.ClusterConfig)
		setupClusterCfg func(*v1beta1.ClusterConfig)
		assertDS        func(*testing.T, *appsv1.DaemonSet)
		assertCM        func(*testing.T, *corev1.ConfigMap)
	}{
		{
			name: "custom config with MTU, peering, hairpin, and ipmasq",
			setupClusterCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.Network.KubeRouter.AutoMTU = ptr.To(false)
				cfg.Spec.Network.KubeRouter.MTU = 1450
				cfg.Spec.Network.KubeRouter.PeerRouterASNs = "12345,67890"
				cfg.Spec.Network.KubeRouter.PeerRouterIPs = "1.2.3.4,4.3.2.1"
				cfg.Spec.Network.KubeRouter.Hairpin = v1beta1.HairpinAllowed
				cfg.Spec.Network.KubeRouter.IPMasq = true
			},
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--peer-router-ips=1.2.3.4,4.3.2.1")
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--peer-router-asns=12345,67890")
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--hairpin-mode=false")
			},
			assertCM: func(t *testing.T, cm *corev1.ConfigMap) {
				p, err := getKubeRouterPlugin(*cm, "bridge")
				require.NoError(t, err)
				assert.InEpsilon(t, 1450, p["mtu"], 0)
				assert.Equal(t, true, p["hairpinMode"])
				assert.Equal(t, true, p["ipMasq"])
			},
		},
		{
			name: "default manifests",
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				assert.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--hairpin-mode=true")
			},
			assertCM: func(t *testing.T, cm *corev1.ConfigMap) {
				p, err := getKubeRouterPlugin(*cm, "bridge")
				require.NoError(t, err)
				assert.NotContains(t, p, "mtu")
				assert.Equal(t, true, p["hairpinMode"])
				assert.Equal(t, false, p["ipMasq"])
			},
		},
		{
			name: "manual MTU",
			setupClusterCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.Network.KubeRouter.AutoMTU = ptr.To(false)
				cfg.Spec.Network.KubeRouter.MTU = 1234
			},
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				assert.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--auto-mtu=false")
			},
			assertCM: func(t *testing.T, cm *corev1.ConfigMap) {
				p, err := getKubeRouterPlugin(*cm, "bridge")
				require.NoError(t, err)
				assert.InEpsilon(t, 1234, p["mtu"], 0)
			},
		},
		{
			name: "extra args",
			setupClusterCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.Network.KubeRouter.ExtraArgs = map[string]string{
					"foo":          "bar",
					"run-firewall": "false",
				}
			},
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				assert.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--run-firewall=false")
				assert.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--foo=bar")
			},
		},
		{
			name: "raw args",
			setupClusterCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.Network.KubeRouter.ExtraArgs = map[string]string{
					"log-level": "debug",
				}
				cfg.Spec.Network.KubeRouter.RawArgs = []string{
					"--log-level=debug",
					"--log-level=debug",
				}
			},
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				// Verify that both extraArgs and rawArgs are present
				args := ds.Spec.Template.Spec.Containers[0].Args[len(ds.Spec.Template.Spec.Containers[0].Args)-2:]
				for _, arg := range args {
					assert.Equal(t, "--log-level=debug", arg)
				}
			},
		},
		{
			name: "with service proxy",
			setupClusterCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.Network.KubeRouter.ExtraArgs = map[string]string{
					"run-service-proxy": "true",
				}
				cfg.Spec.Network.KubeProxy.Disabled = true
			},
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--run-service-proxy=true")
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--master=https://10.0.0.1:6443")
			},
		},
		{
			name: "with service proxy and external address",
			setupNodeCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.API.ExternalAddress = "api.example.com"
			},
			setupClusterCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.Network.KubeRouter.ExtraArgs = map[string]string{
					"run-service-proxy": "true",
				}
				cfg.Spec.Network.KubeProxy.Disabled = true
				cfg.Spec.API.ExternalAddress = "api.example.com"
			},
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--run-service-proxy=true")
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--master=https://api.example.com:6443")
			},
		},
		{
			name: "always sets master",
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--master=https://10.0.0.1:6443")
			},
		},
		{
			name: "with NLLB",
			setupNodeCfg: func(cfg *v1beta1.ClusterConfig) {
				cfg.Spec.Network.NodeLocalLoadBalancing = &v1beta1.NodeLocalLoadBalancing{
					Enabled: true,
					Type:    v1beta1.NllbTypeEnvoyProxy,
					EnvoyProxy: &v1beta1.EnvoyProxy{
						APIServerBindPort: 7443,
					},
				}
			},
			assertDS: func(t *testing.T, ds *appsv1.DaemonSet) {
				require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--master=https://localhost:7443")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestDir := t.TempDir()

			nodeConfig := v1beta1.DefaultClusterConfig()
			// Set default API configuration
			nodeConfig.Spec.API.Port = 6443
			nodeConfig.Spec.API.Address = "10.0.0.1"
			if tt.setupNodeCfg != nil {
				tt.setupNodeCfg(nodeConfig)
			}

			cfg := v1beta1.DefaultClusterConfig()
			cfg.Spec.Network.Calico = nil
			cfg.Spec.Network.Provider = "kuberouter"
			cfg.Spec.Network.KubeRouter = v1beta1.DefaultKubeRouter()
			// Set default API configuration
			cfg.Spec.API.Port = 6443
			cfg.Spec.API.Address = "10.0.0.1"
			if tt.setupClusterCfg != nil {
				tt.setupClusterCfg(cfg)
			}

			ctx := t.Context()
			kr := NewKubeRouter(nodeConfig, manifestDir)
			require.NoError(t, kr.Init(ctx))
			require.NoError(t, kr.Start(ctx))
			t.Cleanup(func() { assert.NoError(t, kr.Stop()) })
			require.NoError(t, kr.Reconcile(ctx, cfg))

			manifestData, err := os.ReadFile(filepath.Join(manifestDir, "kuberouter", "kube-router.yaml"))
			assert.NoError(t, err, "must have manifests for kube-router")

			resources, err := testutil.ParseManifests(manifestData)
			require.NoError(t, err)

			if tt.assertDS != nil {
				ds, err := findDaemonset(resources)
				require.NoError(t, err)
				require.NotNil(t, ds)
				tt.assertDS(t, &ds)
			}

			if tt.assertCM != nil {
				cm, err := findConfig(resources)
				require.NoError(t, err)
				require.NotNil(t, cm)
				tt.assertCM(t, &cm)
			}
		})
	}
}

type hairpinTest struct {
	krc                 *v1beta1.KubeRouter
	resultCNIHairpin    bool
	resultGlobalHairpin bool
}

func TestGetHairpinConfig(t *testing.T) {
	hairpinTests := []hairpinTest{
		{
			krc:                 &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinUndefined, HairpinMode: true},
			resultCNIHairpin:    true,
			resultGlobalHairpin: true,
		},
		{
			krc:                 &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinUndefined, HairpinMode: false},
			resultCNIHairpin:    false,
			resultGlobalHairpin: false,
		},
		{
			krc:                 &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinAllowed, HairpinMode: true},
			resultCNIHairpin:    true,
			resultGlobalHairpin: false,
		},
		{
			krc:                 &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinDisabled, HairpinMode: true},
			resultCNIHairpin:    false,
			resultGlobalHairpin: false,
		},
		{
			krc:                 &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinEnabled, HairpinMode: false},
			resultCNIHairpin:    true,
			resultGlobalHairpin: true,
		},
	}

	for _, test := range hairpinTests {
		cfg := &kubeRouterConfig{}
		cniHairpin, globalHairpin := getHairpinConfig(test.krc)
		if cniHairpin != test.resultCNIHairpin {
			t.Fatalf("CNI hairpin configuration (%#v) does not match exepected output (%#v) ", cfg, test.resultCNIHairpin)
		}
		if globalHairpin != test.resultGlobalHairpin {
			t.Fatalf("Global hairpin configuration (%#v) does not match exepected output (%#v) ", cfg, test.resultGlobalHairpin)
		}

	}
}

func findConfig(resources []*unstructured.Unstructured) (corev1.ConfigMap, error) {
	var cm corev1.ConfigMap
	for _, r := range resources {
		if r.GetKind() == "ConfigMap" {
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(r.Object, &cm)
			if err != nil {
				return cm, err
			}

			return cm, nil
		}
	}

	return cm, errors.New("kube-router cm not found in manifests")
}

func getKubeRouterPlugin(cm corev1.ConfigMap, pluginType string) (map[string]any, error) {
	var data map[string]any
	err := json.Unmarshal([]byte(cm.Data["cni-conf.json"]), &data)
	if err != nil {
		return data, err
	}
	if plugins, ok := data["plugins"].([]any); ok {
		for _, plugin := range plugins {
			if p, ok := plugin.(map[string]any); ok && p["type"] == pluginType {
				return p, nil
			}
		}
	}

	return data, fmt.Errorf("failed to find plugin of type %s", pluginType)
}

func findDaemonset(resources []*unstructured.Unstructured) (appsv1.DaemonSet, error) {
	var ds appsv1.DaemonSet
	for _, r := range resources {
		if r.GetKind() == "DaemonSet" {
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(r.Object, &ds)
			if err != nil {
				return ds, err
			}

			return ds, nil
		}
	}

	return ds, errors.New("kube-router ds not found in manifests")
}
