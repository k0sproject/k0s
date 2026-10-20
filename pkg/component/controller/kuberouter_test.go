// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeRouterManifests(t *testing.T) {
	newClusterConfig := func() *v1beta1.ClusterConfig {
		return &v1beta1.ClusterConfig{Spec: &v1beta1.ClusterSpec{
			Network: v1beta1.DefaultNetwork(),
			Images:  v1beta1.DefaultClusterImages(),
		}}
	}

	reconcile := func(t *testing.T, paf v1beta1.PrimaryAddressFamilyType, cfg *v1beta1.ClusterConfig) (ds *appsv1.DaemonSet, cm *corev1.ConfigMap) {
		k0sVars, err := config.NewCfgVars(nil, t.TempDir())
		require.NoError(t, err)

		serviceCIDRs, singleStackIPv6 := "10.96.0.0/12", false
		if paf == v1beta1.PrimaryFamilyIPv6 {
			serviceCIDRs, singleStackIPv6 = "fd01::/108", true
		}

		kr := NewKubeRouter(k0sVars, paf, serviceCIDRs, singleStackIPv6)
		require.NoError(t, kr.Init(t.Context()))
		require.NoError(t, kr.Start(t.Context()))
		t.Cleanup(func() { assert.NoError(t, kr.Stop()) })
		require.NoError(t, kr.Reconcile(t.Context(), cfg.DeepCopy()))

		f, err := os.Open(filepath.Join(k0sVars.ManifestsDir, "kuberouter", "kube-router.yaml"))
		require.NoError(t, err)
		defer f.Close()

		for obj, err := range testutil.ParseObjects(scheme.Scheme, f) {
			require.NoError(t, err)
			switch obj := obj.(type) {
			case *appsv1.DaemonSet:
				if ds == nil {
					ds = obj
					continue
				}
			case *corev1.ConfigMap:
				if cm == nil {
					cm = obj
					continue
				}
			default:
				continue
			}
			require.Failf(t, "Unexpected object", "%#v", obj)
		}

		require.NotNil(t, ds, "kube-router DaemonSet not found in manifests")
		require.NotNil(t, cm, "kube-router ConfigMap not found in manifests")
		return ds, cm
	}

	requireBridgePlugin := func(t *testing.T, cm *corev1.ConfigMap) map[string]any {
		var data struct {
			Plugins []map[string]any `json:"plugins"`
		}
		require.NoError(t, json.Unmarshal([]byte(cm.Data["cni-conf.json"]), &data))
		for _, plugin := range data.Plugins {
			if plugin["type"] == "bridge" {
				return plugin
			}
		}
		require.Fail(t, "bridge plugin not found in CNI config")
		return nil
	}

	t.Run("defaults", func(t *testing.T) {
		ds, cm := reconcile(t, v1beta1.PrimaryFamilyIPv4, newClusterConfig())

		args := ds.Spec.Template.Spec.Containers[0].Args
		assert.Contains(t, args, "--auto-mtu=true")
		assert.Contains(t, args, "--hairpin-mode=true")
		assert.Contains(t, args, "--enable-ipv4=true")
		assert.Contains(t, args, "--enable-ipv6=false")
		assert.Contains(t, args, "--metrics-port=8080")

		p := requireBridgePlugin(t, cm)
		assert.NotContains(t, p, "mtu")
		assert.Equal(t, true, p["hairpinMode"])
		assert.Equal(t, false, p["ipMasq"])
	})

	t.Run("manual MTU", func(t *testing.T) {
		cfg := newClusterConfig()
		cfg.Spec.Network.KubeRouter.AutoMTU = new(false)
		cfg.Spec.Network.KubeRouter.MTU = 1234

		ds, cm := reconcile(t, v1beta1.PrimaryFamilyIPv4, cfg)

		assert.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--auto-mtu=false")
		assert.InEpsilon(t, 1234, requireBridgePlugin(t, cm)["mtu"], 0)
	})

	t.Run("peer routers, hairpin and IP masquerading", func(t *testing.T) {
		cfg := newClusterConfig()
		cfg.Spec.Network.KubeRouter.AutoMTU = new(false)
		cfg.Spec.Network.KubeRouter.MTU = 1450
		cfg.Spec.Network.KubeRouter.PeerRouterASNs = "12345,67890"
		cfg.Spec.Network.KubeRouter.PeerRouterIPs = "1.2.3.4,4.3.2.1"
		cfg.Spec.Network.KubeRouter.Hairpin = v1beta1.HairpinAllowed
		cfg.Spec.Network.KubeRouter.IPMasq = true

		ds, cm := reconcile(t, v1beta1.PrimaryFamilyIPv4, cfg)

		args := ds.Spec.Template.Spec.Containers[0].Args
		assert.Contains(t, args, "--peer-router-ips=1.2.3.4,4.3.2.1")
		assert.Contains(t, args, "--peer-router-asns=12345,67890")
		assert.Contains(t, args, "--hairpin-mode=false")

		p := requireBridgePlugin(t, cm)
		assert.InEpsilon(t, 1450, p["mtu"], 0)
		assert.Equal(t, true, p["hairpinMode"])
		assert.Equal(t, true, p["ipMasq"])
	})

	t.Run("extra args", func(t *testing.T) {
		cfg := newClusterConfig()
		cfg.Spec.Network.KubeRouter.ExtraArgs = map[string]string{
			"foo":          "bar",   // Add some random arg
			"run-firewall": "false", // Override the default arg
		}

		ds, _ := reconcile(t, v1beta1.PrimaryFamilyIPv6, cfg)

		args := ds.Spec.Template.Spec.Containers[0].Args
		assert.Contains(t, args, "--router-id=generate", "IPv6 related flags not found")
		assert.Contains(t, args, "--run-firewall=false")
		assert.Contains(t, args, "--foo=bar")
	})

	t.Run("raw args", func(t *testing.T) {
		cfg := newClusterConfig()
		cfg.Spec.Network.KubeRouter.ExtraArgs = map[string]string{
			"log-level": "debug",
		}
		cfg.Spec.Network.KubeRouter.RawArgs = []string{
			"--log-level=debug",
			"--log-level=debug",
		}

		ds, _ := reconcile(t, v1beta1.PrimaryFamilyIPv4, cfg)

		// Verify that both extraArgs and rawArgs are present
		args := ds.Spec.Template.Spec.Containers[0].Args
		assert.Equal(t, []string{"--log-level=debug", "--log-level=debug"}, args[len(args)-2:])
	})

	t.Run("address families come from node config", func(t *testing.T) {
		cfg := newClusterConfig()

		ds, _ := reconcile(t, v1beta1.PrimaryFamilyIPv6, cfg)

		args := ds.Spec.Template.Spec.Containers[0].Args
		assert.Contains(t, args, "--enable-ipv4=false")
		assert.Contains(t, args, "--enable-ipv6=true")
	})
}

func TestGetHairpinConfig(t *testing.T) {
	for _, tt := range []struct {
		name                              string
		hairpin                           v1beta1.Hairpin
		hairpinMode                       bool
		wantCNIHairpin, wantGlobalHairpin bool
	}{
		{"undefined with hairpin mode", v1beta1.HairpinUndefined, true, true, true},
		{"undefined without hairpin mode", v1beta1.HairpinUndefined, false, false, false},
		{"allowed", v1beta1.HairpinAllowed, true, true, false},
		{"disabled", v1beta1.HairpinDisabled, true, false, false},
		{"enabled", v1beta1.HairpinEnabled, false, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cniHairpin, globalHairpin := getHairpinConfig(&v1beta1.KubeRouter{
				Hairpin:     tt.hairpin,
				HairpinMode: tt.hairpinMode,
			})
			assert.Equal(t, tt.wantCNIHairpin, cniHairpin, "CNI hairpin")
			assert.Equal(t, tt.wantGlobalHairpin, globalHairpin, "global hairpin")
		})
	}
}
