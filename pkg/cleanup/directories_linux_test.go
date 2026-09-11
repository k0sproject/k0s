//go:build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/mount-utils"
)

// stubSeams replaces the host-touching seams for the duration of a test and
// returns a recorder of the directory removals in call order.
func stubSeams(t *testing.T, mounter mount.Interface, removeErrFor string) *[]string {
	t.Helper()
	var removed []string

	prevMounter, prevRemove := newMounter, removeAll
	t.Cleanup(func() { newMounter, removeAll = prevMounter, prevRemove })

	newMounter = func() mount.Interface { return mounter }
	removeAll = func(path string) error {
		removed = append(removed, path)
		if removeErrFor != "" && path == removeErrFor {
			return errors.New("injected removal failure")
		}
		return nil
	}
	return &removed
}

func testDirs(t *testing.T) *directories {
	t.Helper()
	base := t.TempDir()
	return &directories{
		dataDir:        filepath.Join(base, "data"),
		kubeletRootDir: filepath.Join(base, "kubelet"),
		runDir:         filepath.Join(base, "run"),
	}
}

func unmountedPaths(fake *mount.FakeMounter) []string {
	var paths []string
	for _, action := range fake.GetLog() {
		if action.Action == mount.FakeActionUnmount {
			paths = append(paths, action.Target)
		}
	}
	return paths
}

// Regression test for the first half of https://github.com/k0sproject/k0s/issues/8048:
// mounts under the run dir (containerd task rootfs overlays) were never
// unmounted before the run dir was deleted, unlike mounts under the data dir
// and the kubelet root dir.
func TestRunUnmountsLeftoverMountsUnderEveryManagedDir(t *testing.T) {
	d := testDirs(t)
	taskMount := filepath.Join(d.runDir, "containerd/io.containerd.runtime.v2.task/k8s.io/straggler/rootfs")
	dataMount := filepath.Join(d.dataDir, "containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/1/fs")
	kubeletMount := filepath.Join(d.kubeletRootDir, "pods/x/volumes/kubernetes.io~projected/kube-api-access")
	unrelatedMount := filepath.Join(t.TempDir(), "unrelated")

	fake := mount.NewFakeMounter([]mount.MountPoint{
		{Path: dataMount},
		{Path: kubeletMount},
		{Path: taskMount},
		{Path: unrelatedMount},
	})
	stubSeams(t, fake, "")

	require.NoError(t, d.Run(context.Background()))

	unmounted := unmountedPaths(fake)
	assert.Contains(t, unmounted, taskMount,
		"a container task mount under the run dir must be unmounted before the run dir is deleted")
	assert.Contains(t, unmounted, dataMount)
	assert.Contains(t, unmounted, kubeletMount)
	assert.NotContains(t, unmounted, unrelatedMount,
		"mounts outside the managed directories must be left alone")
}

// Regression test for the second half of https://github.com/k0sproject/k0s/issues/8048:
// the data dir holds the bundled binaries a retried reset needs, so it must be
// deleted last.
func TestRunDeletesDataDirLast(t *testing.T) {
	d := testDirs(t)
	removed := stubSeams(t, mount.NewFakeMounter(nil), "")

	require.NoError(t, d.Run(context.Background()))

	assert.Equal(t, []string{d.kubeletRootDir, d.runDir, d.dataDir}, *removed)
}

// A failure to delete the run dir (e.g. a mount that survived everything else)
// must leave the data dir untouched so the next reset can still extract the
// bundled binaries and retry.
func TestRunPreservesDataDirWhenRunDirDeletionFails(t *testing.T) {
	d := testDirs(t)
	removed := stubSeams(t, mount.NewFakeMounter(nil), d.runDir)

	err := d.Run(context.Background())

	require.ErrorContains(t, err, d.runDir)
	assert.NotContains(t, *removed, d.dataDir,
		"the data dir must survive a failed run dir deletion")
}

// The non-root layout nests the run dir inside the data dir (<dataDir>/run).
// Mounts under it must still be unmounted, and the deletion order must hold
// with the nested path.
func TestRunHandlesRunDirNestedInDataDir(t *testing.T) {
	base := t.TempDir()
	d := &directories{
		dataDir:        filepath.Join(base, "data"),
		kubeletRootDir: filepath.Join(base, "data", "kubelet"),
		runDir:         filepath.Join(base, "data", "run"),
	}
	taskMount := filepath.Join(d.runDir, "containerd/io.containerd.runtime.v2.task/k8s.io/straggler/rootfs")

	fake := mount.NewFakeMounter([]mount.MountPoint{{Path: taskMount}})
	removed := stubSeams(t, fake, "")

	require.NoError(t, d.Run(context.Background()))

	assert.Contains(t, unmountedPaths(fake), taskMount)
	assert.Equal(t, []string{d.kubeletRootDir, d.runDir, d.dataDir}, *removed)
}
