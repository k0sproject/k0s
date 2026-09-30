// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	kubeproxyv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
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

	synctest.Test(t, func(t *testing.T) {

		_, manifestsDir := startComponent(t, cfg)
		_, _, configMap := awaitUpdate(t, manifestsDir, nil)

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
	})
}

func TestKubeProxyConfig_HashChangesWhenConfigMapChanges(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig()

	synctest.Test(t, func(t *testing.T) {
		underTest, manifestsDir := startComponent(t, cfg)

		stat, initialDaemonSet, _ := awaitUpdate(t, manifestsDir, nil)

		cfg.Spec.FeatureGates = v1beta1.FeatureGates{
			{Name: "Feature0", Enabled: true, Components: []string{"kube-proxy"}},
		}
		require.NoError(t, underTest.Reconcile(t.Context(), cfg))

		stat, updatedDaemonSet, _ := awaitUpdate(t, manifestsDir, stat)
		require.NotNil(t, stat, "Manifest file wasn't reconciled")

		assert.NotEqual(t,
			initialDaemonSet.Spec.Template.Annotations["k0sproject.io/config-hash"],
			updatedDaemonSet.Spec.Template.Annotations["k0sproject.io/config-hash"],
		)
	})
}

func startComponent(t *testing.T, cfg *v1beta1.ClusterConfig) (*KubeProxy, string) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	noWindowsNodes := func() (*bool, <-chan struct{}) {
		return new(false), nil
	}

	underTest := NewKubeProxy(k0sVars, cfg, noWindowsNodes)
	require.NoError(t, underTest.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, underTest.Reconcile(t.Context(), cfg))

	return underTest, k0sVars.ManifestsDir
}

func awaitUpdate(t *testing.T, manifestsDir string, prev os.FileInfo) (_ os.FileInfo, ds *appsv1.DaemonSet, cm *corev1.ConfigMap) {
	manifestPath := filepath.Join(manifestsDir, "kubeproxy", "kube-proxy.yaml")

	synctest.Wait()
	manifestFile, err := os.Open(manifestPath)
	require.NoError(t, err)
	defer manifestFile.Close()

	stat, err := manifestFile.Stat()
	require.NoError(t, err)
	if prev != nil && prev.ModTime().Equal(stat.ModTime()) && prev.Size() == stat.Size() {
		return nil, nil, nil
	}

	for obj, err := range testutil.ParseObjects(scheme.Scheme, manifestFile) {
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

	require.NotNil(t, ds, "kube-proxy DaemonSet not found in manifests")
	require.NotNil(t, cm, "kube-proxy ConfigMap not found in manifests")
	return stat, ds, cm
}
