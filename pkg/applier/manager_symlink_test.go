// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManager_WithSymlinkedStacks(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create k0s variables
	k0sVars, err := config.NewCfgVars(nil, tempDir)
	require.NoError(t, err)
	
	// Create a real stack directory outside the manifests dir
	realStackDir := filepath.Join(tempDir, "real-stacks", "my-stack")
	require.NoError(t, os.MkdirAll(realStackDir, 0755))
	
	// Create manifest files in the real directory
	manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: symlink-test
  namespace: default
  resourceVersion: "1"
data:
  key: value
`
	require.NoError(t, os.WriteFile(filepath.Join(realStackDir, "config.yaml"), []byte(manifestContent), constant.CertMode))
	
	// Setup manager
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()
	
	underTest := applier.Manager{
		K0sVars:           k0sVars,
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}
	
	// Initialize the manager to create the manifests directory
	require.NoError(t, underTest.Init(t.Context()))
	
	// Create a symlink in the manifests directory pointing to the real stack
	symlinkPath := filepath.Join(k0sVars.ManifestsDir, "my-stack-symlink")
	require.NoError(t, os.Symlink(realStackDir, symlinkPath))
	
	// Start the manager
	require.NoError(t, leaderElector.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })
	
	// Wait for the symlinked stack to be applied
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps(metav1.NamespaceDefault)).
		Until(t.Context(), func(item *corev1.ConfigMap) (bool, error) {
			return item.Name == "symlink-test", nil
		}),
	)
}

func TestManager_WithSymlinkedStackAfterStart(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create k0s variables
	k0sVars, err := config.NewCfgVars(nil, tempDir)
	require.NoError(t, err)
	
	// Setup manager
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()
	
	underTest := applier.Manager{
		K0sVars:           k0sVars,
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}
	
	// Start the manager first
	require.NoError(t, leaderElector.Init(t.Context()))
	require.NoError(t, underTest.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })
	
	// Create a real stack directory after the manager is started
	realStackDir := filepath.Join(tempDir, "real-stacks", "after-start")
	require.NoError(t, os.MkdirAll(realStackDir, 0755))
	
	// Create manifest files in the real directory
	manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: after-start-symlink
  namespace: default
  resourceVersion: "1"
data:
  key: value
`
	require.NoError(t, os.WriteFile(filepath.Join(realStackDir, "config.yaml"), []byte(manifestContent), constant.CertMode))
	
	// Create a symlink in the manifests directory pointing to the real stack
	symlinkPath := filepath.Join(k0sVars.ManifestsDir, "after-start-symlink")
	require.NoError(t, os.Symlink(realStackDir, symlinkPath))
	
	// Wait for the symlinked stack to be applied
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps(metav1.NamespaceDefault)).
		Until(t.Context(), func(item *corev1.ConfigMap) (bool, error) {
			return item.Name == "after-start-symlink", nil
		}),
	)
}

func TestManager_WithBrokenSymlink(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create k0s variables
	k0sVars, err := config.NewCfgVars(nil, tempDir)
	require.NoError(t, err)
	
	// Setup manager
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()
	
	underTest := applier.Manager{
		K0sVars:           k0sVars,
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}
	
	// Initialize the manager to create the manifests directory
	require.NoError(t, underTest.Init(t.Context()))
	
	// Create a broken symlink (points to non-existent directory)
	brokenSymlinkPath := filepath.Join(k0sVars.ManifestsDir, "broken-symlink")
	require.NoError(t, os.Symlink("/nonexistent/path", brokenSymlinkPath))
	
	// Start the manager - should not fail with broken symlinks
	require.NoError(t, leaderElector.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })
	
	// Wait a bit to ensure no errors occur
	time.Sleep(100 * time.Millisecond)
	
	// Verify no configmaps were created (since the symlink is broken)
	configMaps, err := clients.Client.CoreV1().ConfigMaps(metav1.NamespaceDefault).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, configMaps.Items)
}

func TestManager_WithSymlinkedManifestFile(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create k0s variables
	k0sVars, err := config.NewCfgVars(nil, tempDir)
	require.NoError(t, err)
	
	// Create a regular stack directory
	stackDir := filepath.Join(k0sVars.ManifestsDir, "regular-stack")
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	
	// Create a real manifest file outside the stack directory
	realManifestPath := filepath.Join(tempDir, "real-manifests", "external.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(realManifestPath), 0755))
	manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: external-manifest
  namespace: default
  resourceVersion: "1"
data:
  key: external-value
`
	require.NoError(t, os.WriteFile(realManifestPath, []byte(manifestContent), constant.CertMode))
	
	// Create a symlink to the manifest file inside the stack directory
	symlinkManifestPath := filepath.Join(stackDir, "external.yaml")
	require.NoError(t, os.Symlink(realManifestPath, symlinkManifestPath))
	
	// Setup manager
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()
	
	underTest := applier.Manager{
		K0sVars:           k0sVars,
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}
	
	// Start the manager
	require.NoError(t, leaderElector.Init(t.Context()))
	require.NoError(t, underTest.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })
	
	// Wait for the symlinked manifest to be applied
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps(metav1.NamespaceDefault)).
		Until(t.Context(), func(item *corev1.ConfigMap) (bool, error) {
			return item.Name == "external-manifest", nil
		}),
	)
}
