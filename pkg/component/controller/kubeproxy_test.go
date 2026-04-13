// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

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
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.FeatureGates = v1beta1.FeatureGates{
		{Name: "Feature0", Enabled: true, Components: []string{"kube-proxy"}},
		{Name: "Feature1", Enabled: false, Components: []string{"kube-proxy"}},
	}
	noWindowsNodes := func() (*bool, <-chan struct{}) {
		return ptr.To(false), nil
	}

	underTest := NewKubeProxy(k0sVars, cfg, noWindowsNodes)
	require.NoError(t, underTest.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, underTest.Reconcile(t.Context(), cfg))

	manifestPath := filepath.Join(k0sVars.ManifestsDir, "kubeproxy", "kube-proxy.yaml")
	var manifestData []byte
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		manifestData, err = os.ReadFile(manifestPath)
		assert.NoError(t, err)
	}, 5*time.Second, 50*time.Millisecond)

	resources, err := testutil.ParseManifests(manifestData)
	require.NoError(t, err)

	configMapIdx := slices.IndexFunc(resources, func(u *unstructured.Unstructured) bool {
		return u.GetAPIVersion() == corev1.SchemeGroupVersion.String() &&
			u.GetKind() == "ConfigMap" &&
			u.GetNamespace() == "kube-system" &&
			u.GetName() == "kube-proxy"
	})
	require.GreaterOrEqual(t, configMapIdx, 0, "kube-proxy ConfigMap not found")
	var cm corev1.ConfigMap
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(resources[configMapIdx].Object, &cm))

	var kubeProxyConfigData unstructured.Unstructured
	require.NoError(t, yaml.Unmarshal([]byte(cm.Data["config.conf"]), &kubeProxyConfigData.Object))
	assert.Equal(t, kubeproxyv1alpha1.SchemeGroupVersion.String(), kubeProxyConfigData.GetAPIVersion())
	assert.Equal(t, "KubeProxyConfiguration", kubeProxyConfigData.GetKind())

	renderedFeatureGates, ok := kubeProxyConfigData.Object["featureGates"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"Feature0": true,
		"Feature1": false,
	}, renderedFeatureGates)
}
