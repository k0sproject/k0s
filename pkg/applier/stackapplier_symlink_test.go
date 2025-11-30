// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/applier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStackApplier_WithSymlinkedDirectory(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a real manifest directory
	realDir := filepath.Join(tempDir, "real-manifests")
	require.NoError(t, os.MkdirAll(realDir, 0755))
	
	// Create manifest files in the real directory
	manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: symlink-dir-test
  namespace: default
data:
  key: value
`
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "config.yaml"), []byte(manifestContent), 0644))
	
	// Create a symlink directory
	symlinkDir := filepath.Join(tempDir, "symlink-manifests")
	require.NoError(t, os.Symlink(realDir, symlinkDir))
	
	// Create applier with the symlink directory
	clients := testutil.NewFakeClientFactory()
	applier := applier.NewApplier(symlinkDir, clients)
	
	// Apply the stack
	ctx := t.Context()
	err := applier.Apply(ctx)
	require.NoError(t, err)
	
	// Verify the configmap was created
	configMap, err := clients.Client.CoreV1().ConfigMaps("default").Get(ctx, "symlink-dir-test", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "symlink-dir-test", configMap.Name)
	assert.Equal(t, "value", configMap.Data["key"])
}

func TestStackApplier_WithSymlinkedFiles(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a manifest directory
	manifestDir := filepath.Join(tempDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))
	
	// Create real manifest files outside the directory
	realDir := filepath.Join(tempDir, "real")
	require.NoError(t, os.MkdirAll(realDir, 0755))
	
	manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: symlink-file-test
  namespace: default
data:
  key: value
`
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "real.yaml"), []byte(manifestContent), 0644))
	
	// Create symlinks to the manifest files
	symlinkPath := filepath.Join(manifestDir, "symlink.yaml")
	require.NoError(t, os.Symlink(filepath.Join(realDir, "real.yaml"), symlinkPath))
	
	// Create applier with the directory containing symlinks
	clients := testutil.NewFakeClientFactory()
	applier := applier.NewApplier(manifestDir, clients)
	
	// Apply the stack
	ctx := t.Context()
	err := applier.Apply(ctx)
	require.NoError(t, err)
	
	// Verify the configmap was created
	configMap, err := clients.Client.CoreV1().ConfigMaps("default").Get(ctx, "symlink-file-test", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "symlink-file-test", configMap.Name)
	assert.Equal(t, "value", configMap.Data["key"])
}

func TestStackApplier_WithBrokenSymlink(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a manifest directory
	manifestDir := filepath.Join(tempDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))
	
	// Create a broken symlink
	brokenSymlinkPath := filepath.Join(manifestDir, "broken.yaml")
	require.NoError(t, os.Symlink("/nonexistent/path", brokenSymlinkPath))
	
	// Create applier with the directory containing broken symlink
	clients := testutil.NewFakeClientFactory()
	applier := applier.NewApplier(manifestDir, clients)
	
	// Apply the stack - should not fail with broken symlinks
	ctx := t.Context()
	err := applier.Apply(ctx)
	require.NoError(t, err)
	
	// Verify no configmaps were created
	configMaps, err := clients.Client.CoreV1().ConfigMaps("default").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, configMaps.Items)
}

func TestStackApplier_WithMixedFilesAndSymlinks(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a manifest directory
	manifestDir := filepath.Join(tempDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))
	
	// Create a regular manifest file
	regularContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: regular-config
  namespace: default
data:
  type: regular
`
	require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "regular.yaml"), []byte(regularContent), 0644))
	
	// Create real manifest file outside the directory
	realDir := filepath.Join(tempDir, "real")
	require.NoError(t, os.MkdirAll(realDir, 0755))
	
	symlinkContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: symlink-config
  namespace: default
data:
  type: symlink
`
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "external.yaml"), []byte(symlinkContent), 0644))
	
	// Create symlink to the external manifest
	symlinkPath := filepath.Join(manifestDir, "symlink.yaml")
	require.NoError(t, os.Symlink(filepath.Join(realDir, "external.yaml"), symlinkPath))
	
	// Create applier with the directory containing both regular and symlinked files
	clients := testutil.NewFakeClientFactory()
	applier := applier.NewApplier(manifestDir, clients)
	
	// Apply the stack
	ctx := t.Context()
	err := applier.Apply(ctx)
	require.NoError(t, err)
	
	// Verify both configmaps were created
	regularConfig, err := clients.Client.CoreV1().ConfigMaps("default").Get(ctx, "regular-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "regular", regularConfig.Data["type"])
	
	symlinkConfig, err := clients.Client.CoreV1().ConfigMaps("default").Get(ctx, "symlink-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "symlink", symlinkConfig.Data["type"])
}
