/*
Copyright 2020 k0s authors

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

package controller_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_KubeletConfig_Nonexistent(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "manifests", "kubelet")

	k0sVars, err := config.NewCfgVars(nil, tmp)
	require.NoError(t, err)

	underTest := controller.NewKubeletConfig(k0sVars)
	assert.NoError(t, underTest.Init(context.TODO()))
	assert.DirExists(t, dir, "Kubelet manifest directory wasn't created")
}

func Test_KubeletConfig_ManifestDirObstructed(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "manifests")
	require.NoError(t, os.WriteFile(dir, []byte("obstructed"), 0644))

	k0sVars, err := config.NewCfgVars(nil, tmp)
	require.NoError(t, err)

	underTest := controller.NewKubeletConfig(k0sVars)
	err = underTest.Init(context.TODO())

	if pathErr := (*os.PathError)(nil); assert.ErrorAs(t, err, &pathErr) {
		assert.Equal(t, pathErr.Path, dir)
		assert.Equal(t, pathErr.Op, "mkdir")
		assert.Equal(t, pathErr.Err, syscall.ENOTDIR)
	}
}

func Test_KubeletConfig_RenamesManifestFiles(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "manifests", "kubelet")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deprecated.txt"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yaml"), nil, 0644))

	foundFiles, err := applier.FindManifestFilesInDir(dir)
	require.NoError(t, err, "Failed to find manifests in kubelet dir")
	require.NotEmpty(t, foundFiles, "No manifests in kubelet dir; did the applier change its file patterns?")

	k0sVars, err := config.NewCfgVars(nil, tmp)
	require.NoError(t, err)

	underTest := controller.NewKubeletConfig(k0sVars)
	assert.NoError(t, underTest.Init(context.TODO()))

	assert.NoFileExists(t, filepath.Join(dir, "deprecated.txt"))
	assert.FileExists(t, filepath.Join(dir, "removed.txt"))
	if matches, err := filepath.Glob(filepath.Join(dir, "manifest.yaml.*.removed")); assert.NoError(t, err) {
		assert.Len(t, matches, 1, "Expected a single removed manifest file")
	}
	if entries, err := os.ReadDir(dir); assert.NoError(t, err) {
		assert.Len(t, entries, 2, "Expected exactly two files in kubelet folder")
	}

	foundFiles, err = applier.FindManifestFilesInDir(dir)
	if assert.NoError(t, err, "Failed to find manifests in kubelet dir") {
		assert.Empty(t, foundFiles, "Some manifests were still found in kubelet dir")
	}
}
