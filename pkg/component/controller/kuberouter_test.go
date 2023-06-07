/*
Copyright 2021 k0s authors

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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestKubeRouterConfig(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.Network.Calico = nil
	cfg.Spec.Network.Provider = "kuberouter"
	cfg.Spec.Network.KubeRouter = v1beta1.DefaultKubeRouter()
	cfg.Spec.Network.KubeRouter.AutoMTU = false
	cfg.Spec.Network.KubeRouter.MTU = 1450
	cfg.Spec.Network.KubeRouter.PeerRouterASNs = "12345,67890"
	cfg.Spec.Network.KubeRouter.PeerRouterIPs = "1.2.3.4,4.3.2.1"
	cfg.Spec.Network.KubeRouter.Hairpin = v1beta1.HairpinAllowed
	cfg.Spec.Network.KubeRouter.IPMasq = true

	saver := inMemorySaver{}
	kr := NewKubeRouter(k0sVars, saver)
	require.NoError(t, kr.Reconcile(context.Background(), cfg))
	require.NoError(t, kr.Stop())

	manifestData, foundRaw := saver["kube-router.yaml"]
	require.True(t, foundRaw, "must have manifests for kube-router")

	resources, err := testutil.ParseManifests(manifestData)
	require.NoError(t, err)
	ds, err := findDaemonset(resources)
	require.NoError(t, err)
	require.NotNil(t, ds)
	require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--peer-router-ips=1.2.3.4,4.3.2.1")
	require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--peer-router-asns=12345,67890")
	require.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--hairpin-mode=false")

	cm, err := findConfig(resources)
	require.NoError(t, err)
	require.NotNil(t, cm)

	p, err := getKubeRouterPlugin(cm, "bridge")
	require.NoError(t, err)
	require.Equal(t, float64(1450), p.Dig("mtu"))
	require.Equal(t, true, p.Dig("hairpinMode"))
	require.Equal(t, true, p.Dig("ipMasq"))
}

type hairpinTest struct {
	krc    *v1beta1.KubeRouter
	result kubeRouterConfig
}

func TestGetHairpinConfig(t *testing.T) {
	hairpinTests := []hairpinTest{
		{
			krc:    &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinUndefined, HairpinMode: true},
			result: kubeRouterConfig{CNIHairpin: true, GlobalHairpin: true},
		},
		{
			krc:    &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinUndefined, HairpinMode: false},
			result: kubeRouterConfig{CNIHairpin: false, GlobalHairpin: false},
		},
		{
			krc:    &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinAllowed, HairpinMode: true},
			result: kubeRouterConfig{CNIHairpin: true, GlobalHairpin: false},
		},
		{
			krc:    &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinDisabled, HairpinMode: true},
			result: kubeRouterConfig{CNIHairpin: false, GlobalHairpin: false},
		},
		{
			krc:    &v1beta1.KubeRouter{Hairpin: v1beta1.HairpinEnabled, HairpinMode: false},
			result: kubeRouterConfig{CNIHairpin: true, GlobalHairpin: true},
		},
	}

	for _, test := range hairpinTests {
		cfg := &kubeRouterConfig{}
		getHairpinConfig(cfg, test.krc)
		if cfg.CNIHairpin != test.result.CNIHairpin || cfg.GlobalHairpin != test.result.GlobalHairpin {
			t.Fatalf("Hairpin configuration (%#v) does not match exepected output (%#v) ", cfg, test.result)
		}
	}
}

func TestKubeRouterDefaultManifests(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.Network.Calico = nil
	cfg.Spec.Network.Provider = "kuberouter"
	cfg.Spec.Network.KubeRouter = v1beta1.DefaultKubeRouter()
	saver := inMemorySaver{}
	kr := NewKubeRouter(k0sVars, saver)
	require.NoError(t, kr.Reconcile(context.Background(), cfg))
	require.NoError(t, kr.Stop())

	manifestData, foundRaw := saver["kube-router.yaml"]
	require.True(t, foundRaw, "must have manifests for kube-router")

	resources, err := testutil.ParseManifests(manifestData)
	require.NoError(t, err)
	ds, err := findDaemonset(resources)
	require.NoError(t, err)
	require.NotNil(t, ds)

	assert.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--hairpin-mode=true")

	cm, err := findConfig(resources)
	require.NoError(t, err)
	require.NotNil(t, cm)

	p, err := getKubeRouterPlugin(cm, "bridge")
	require.NoError(t, err)
	require.Nil(t, p.Dig("mtu"))
	require.Equal(t, true, p.Dig("hairpinMode"))
	require.Equal(t, false, p.Dig("ipMasq"))
}

func TestKubeRouterManualMTUManifests(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.Network.Calico = nil
	cfg.Spec.Network.Provider = "kuberouter"
	cfg.Spec.Network.KubeRouter = v1beta1.DefaultKubeRouter()
	cfg.Spec.Network.KubeRouter.AutoMTU = false
	cfg.Spec.Network.KubeRouter.MTU = 1234
	saver := inMemorySaver{}
	kr := NewKubeRouter(k0sVars, saver)
	require.NoError(t, kr.Reconcile(context.Background(), cfg))
	require.NoError(t, kr.Stop())

	manifestData, foundRaw := saver["kube-router.yaml"]
	require.True(t, foundRaw, "must have manifests for kube-router")

	resources, err := testutil.ParseManifests(manifestData)
	require.NoError(t, err)
	ds, err := findDaemonset(resources)
	require.NoError(t, err)
	require.NotNil(t, ds)

	assert.Contains(t, ds.Spec.Template.Spec.Containers[0].Args, "--auto-mtu=false")

	cm, err := findConfig(resources)
	require.NoError(t, err)
	require.NotNil(t, cm)

	p, err := getKubeRouterPlugin(cm, "bridge")
	require.NoError(t, err)
	require.Equal(t, float64(1234), p.Dig("mtu"))
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

	return cm, fmt.Errorf("kube-router cm not found in manifests")
}

func getKubeRouterPlugin(cm corev1.ConfigMap, pluginType string) (dig.Mapping, error) {
	data := dig.Mapping{}
	err := json.Unmarshal([]byte(cm.Data["cni-conf.json"]), &data)
	if err != nil {
		return data, err
	}
	plugins, ok := data.Dig("plugins").([]interface{})
	if !ok {
		return data, fmt.Errorf("failed to dig plugins")
	}
	for _, p := range plugins {
		plugin := dig.Mapping(p.(map[string]interface{}))
		if plugin.DigString("type") == pluginType {
			return plugin, nil
		}
	}

	return data, fmt.Errorf("failed to find plugin of type %s", pluginType)
}

func findDaemonset(resources []*unstructured.Unstructured) (v1.DaemonSet, error) {
	var ds v1.DaemonSet
	for _, r := range resources {
		if r.GetKind() == "DaemonSet" {
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(r.Object, &ds)
			if err != nil {
				return ds, err
			}

			return ds, nil
		}
	}

	return ds, fmt.Errorf("kube-router ds not found in manifests")
}
