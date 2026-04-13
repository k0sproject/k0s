// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeproxyv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeProxyConfig_FeatureGates(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.FeatureGates = v1beta1.FeatureGates{
		{Name: "Feature0", Enabled: true, Components: []string{"kube-proxy"}},
		{Name: "Feature1", Enabled: false, Components: []string{"kube-proxy"}},
	}

	_, manifestsDir := startComponent(t, cfg)
	_, resources := awaitUpdate(t, manifestsDir, nil)
	configMap := findKubeProxyConfigMap(t, resources)

	var kubeProxyConfigData unstructured.Unstructured
	require.NoError(t, yaml.Unmarshal([]byte(configMap.Data["config.conf"]), &kubeProxyConfigData.Object))
	assert.Equal(t, kubeproxyv1alpha1.SchemeGroupVersion.String(), kubeProxyConfigData.GetAPIVersion())
	assert.Equal(t, "KubeProxyConfiguration", kubeProxyConfigData.GetKind())

	renderedFeatureGates, ok := kubeProxyConfigData.Object["featureGates"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"Feature0": true,
		"Feature1": false,
	}, renderedFeatureGates)
}

func TestKubeProxyConfig_HashChangesWhenConfigMapChanges(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig()
	underTest, manifestsDir := startComponent(t, cfg)

	stat, resources := awaitUpdate(t, manifestsDir, nil)
	initialDaemonSet := findKubeProxyDaemonSet(t, resources)

	cfg.Spec.FeatureGates = v1beta1.FeatureGates{
		{Name: "Feature0", Enabled: true, Components: []string{"kube-proxy"}},
	}
	require.NoError(t, underTest.Reconcile(t.Context(), cfg))

	_, resources = awaitUpdate(t, manifestsDir, stat)
	updatedDaemonSet := findKubeProxyDaemonSet(t, resources)

	assert.NotEqual(t, initialDaemonSet.Spec.Template.Annotations["k0sproject.io/config-hash"], updatedDaemonSet.Spec.Template.Annotations["k0sproject.io/config-hash"])
}

func startComponent(t *testing.T, cfg *v1beta1.ClusterConfig) (*KubeProxy, string) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	noWindowsNodes := func() (*bool, <-chan struct{}) {
		return ptr.To(false), nil
	}

	underTest := NewKubeProxy(k0sVars, cfg, noWindowsNodes)
	require.NoError(t, underTest.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, underTest.Reconcile(t.Context(), cfg))

	return underTest, k0sVars.ManifestsDir
}

func awaitUpdate(t *testing.T, manifestsDir string, prev os.FileInfo) (os.FileInfo, []*unstructured.Unstructured) {
	manifestPath := filepath.Join(manifestsDir, "kubeproxy", "kube-proxy.yaml")

	var (
		stat         os.FileInfo
		manifestData []byte
	)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		f, err := os.OpenFile(manifestPath, os.O_RDONLY, 0)
		if !assert.NoError(t, err) {
			return
		}
		defer f.Close()

		stat, err = f.Stat()
		if !assert.NoError(t, err) {
			return
		}

		if prev != nil && !assert.True(t, prev.ModTime().Before(stat.ModTime()) || prev.Size() != stat.Size(), "manifest unchanged") {
			return
		}

		manifestData, err = io.ReadAll(f)
		if !assert.NoError(t, err) || !assert.Len(t, manifestData, int(stat.Size())) {
			return
		}
	}, 5*time.Second, 50*time.Millisecond)

	resources, err := testutil.ParseManifests(manifestData)
	require.NoError(t, err)
	return stat, resources
}

func findKubeProxyConfigMap(t *testing.T, resources []*unstructured.Unstructured) *corev1.ConfigMap {
	t.Helper()

	idx := slices.IndexFunc(resources, func(u *unstructured.Unstructured) bool {
		return u.GetAPIVersion() == corev1.SchemeGroupVersion.String() &&
			u.GetKind() == "ConfigMap" &&
			u.GetNamespace() == "kube-system" &&
			u.GetName() == "kube-proxy"
	})
	require.GreaterOrEqual(t, idx, 0, "kube-proxy ConfigMap not found")

	var configMap corev1.ConfigMap
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(resources[idx].Object, &configMap))

	return &configMap
}

func findKubeProxyDaemonSet(t *testing.T, resources []*unstructured.Unstructured) *appsv1.DaemonSet {
	t.Helper()

	idx := slices.IndexFunc(resources, func(u *unstructured.Unstructured) bool {
		return u.GetAPIVersion() == appsv1.SchemeGroupVersion.String() &&
			u.GetKind() == "DaemonSet" &&
			u.GetNamespace() == "kube-system" &&
			u.GetName() == "kube-proxy"
	})
	require.GreaterOrEqual(t, idx, 0, "kube-proxy DaemonSet not found")

	var daemonSet appsv1.DaemonSet
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(resources[idx].Object, &daemonSet))
	return &daemonSet
}
