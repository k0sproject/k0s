// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/k0sproject/k0s/pkg/applier"
)

func TestFindManifestFilesInDir_WithSymlinks(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a real manifest directory
	realDir := filepath.Join(tempDir, "real")
	require.NoError(t, os.MkdirAll(realDir, 0755))
	
	// Create a manifest file in the real directory
	manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  namespace: default
data:
  key: value
`
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "test.yaml"), []byte(manifestContent), 0644))
	
	// Create a symlink directory that points to the real directory
	symlinkDir := filepath.Join(tempDir, "symlink")
	require.NoError(t, os.Symlink(realDir, symlinkDir))
	
	// Test finding files in the symlink directory
	files, err := applier.FindManifestFilesInDir(symlinkDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0], "test.yaml")
}

func TestFindManifestFilesInDir_WithSymlinkedFiles(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a manifest directory
	manifestDir := filepath.Join(tempDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))
	
	// Create a real manifest file
	realManifest := filepath.Join(tempDir, "real", "real.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(realManifest), 0755))
	manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: real
  namespace: default
data:
  key: value
`
	require.NoError(t, os.WriteFile(realManifest, []byte(manifestContent), 0644))
	
	// Create a symlink to the manifest file
	symlinkManifest := filepath.Join(manifestDir, "symlink.yaml")
	require.NoError(t, os.Symlink(realManifest, symlinkManifest))
	
	// Test finding files in the directory with symlinked files
	files, err := applier.FindManifestFilesInDir(manifestDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0], "symlink.yaml")
}

func TestFindManifestFilesInDir_BrokenSymlink(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a manifest directory
	manifestDir := filepath.Join(tempDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))
	
	// Create a broken symlink (points to non-existent file)
	brokenSymlink := filepath.Join(manifestDir, "broken.yaml")
	require.NoError(t, os.Symlink("/nonexistent/path/broken.yaml", brokenSymlink))
	
	// Test finding files - should not include broken symlinks
	files, err := applier.FindManifestFilesInDir(manifestDir)
	require.NoError(t, err)
	assert.Len(t, files, 0)
}

func TestFindManifestFilesInDir_MixedFilesAndSymlinks(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a manifest directory
	manifestDir := filepath.Join(tempDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))
	
	// Create a regular manifest file
	regularManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: regular
  namespace: default
data:
  key: value
`
	require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "regular.yaml"), []byte(regularManifest), 0644))
	
	// Create a real manifest file outside the directory
	realManifest := filepath.Join(tempDir, "real", "real.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(realManifest), 0755))
	symlinkContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: symlinked
  namespace: default
data:
  key: value
`
	require.NoError(t, os.WriteFile(realManifest, []byte(symlinkContent), 0644))
	
	// Create a symlink to the manifest file
	symlinkManifest := filepath.Join(manifestDir, "symlink.yaml")
	require.NoError(t, os.Symlink(realManifest, symlinkManifest))
	
	// Test finding files - should include both regular and symlinked files
	files, err := applier.FindManifestFilesInDir(manifestDir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	
	// Verify both files are found
	var foundRegular, foundSymlink bool
	for _, file := range files {
		if filepath.Base(file) == "regular.yaml" {
			foundRegular = true
		} else if filepath.Base(file) == "symlink.yaml" {
			foundSymlink = true
		}
	}
	assert.True(t, foundRegular, "Regular file not found")
	assert.True(t, foundSymlink, "Symlinked file not found")
}
