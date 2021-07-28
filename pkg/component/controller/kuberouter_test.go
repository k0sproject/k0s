package controller

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestKubeRouterConfig(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig(dataDir)
	cfg.Spec.Network.Calico = nil
	cfg.Spec.Network.Provider = "kuberouter"
	cfg.Spec.Network.KubeRouter = v1beta1.DefaultKubeRouter()
	cfg.Spec.Network.KubeRouter.AutoMTU = false
	cfg.Spec.Network.KubeRouter.MTU = 1450
	cfg.Spec.Network.KubeRouter.PeerRouterASNs = "12345,67890"
	cfg.Spec.Network.KubeRouter.PeerRouterIPs = "1.2.3.4,4.3.2.1"

	saver := inMemorySaver{}
	kr, err := NewKubeRouter(cfg, saver)
	require.NoError(t, err)
	require.NoError(t, kr.Run())
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

	cm, err := findConfig(resources)
	require.NoError(t, err)
	require.NotNil(t, cm)

	p, err := getKubeRouterPlugin(cm, "bridge")
	require.NoError(t, err)
	require.Equal(t, false, p.Dig("auto-mtu"))
	require.Equal(t, float64(1450), p.Dig("mtu"))
}

func TestKubeRouterDefaultManifests(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig(dataDir)
	cfg.Spec.Network.Calico = nil
	cfg.Spec.Network.Provider = "kuberouter"
	cfg.Spec.Network.KubeRouter = v1beta1.DefaultKubeRouter()
	saver := inMemorySaver{}
	kr, err := NewKubeRouter(cfg, saver)
	require.NoError(t, err)
	require.NoError(t, kr.Run())
	require.NoError(t, kr.Stop())

	manifestData, foundRaw := saver["kube-router.yaml"]
	require.True(t, foundRaw, "must have manifests for kube-router")

	resources, err := testutil.ParseManifests(manifestData)
	require.NoError(t, err)
	ds, err := findDaemonset(resources)
	require.NoError(t, err)
	require.NotNil(t, ds)

	cm, err := findConfig(resources)
	require.NoError(t, err)
	require.NotNil(t, cm)

	p, err := getKubeRouterPlugin(cm, "bridge")
	require.NoError(t, err)
	require.Equal(t, true, p.Dig("auto-mtu"))
	require.Nil(t, p.Dig("mtu"))
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
