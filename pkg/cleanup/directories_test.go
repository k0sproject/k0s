//go:build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/mount-utils"
)

// blockingMounter never returns from Unmount, standing in for a umount(2) that
// wedges in D state on a dead or frozen backend.
type blockingMounter struct{ mount.Interface }

func (blockingMounter) Unmount(string) error { select {} }

// okMounter unmounts cleanly.
type okMounter struct{ mount.Interface }

func (okMounter) Unmount(string) error { return nil }

// errMounter fails fast, as a busy mount would.
type errMounter struct{ mount.Interface }

func (errMounter) Unmount(string) error { return errors.New("target is busy") }

// TestDirectories_unmount_boundedByTimeout proves a wedged umount cannot hang
// reset. The caller gets an error and falls back to a lazy detach.
func TestDirectories_unmount_boundedByTimeout(t *testing.T) {
	d := &directories{unmountTimeout: 50 * time.Millisecond}

	start := time.Now()
	err := d.unmount(blockingMounter{}, "/var/lib/kubelet/pods/frozen")

	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestDirectories_unmount_passesResult(t *testing.T) {
	d := &directories{unmountTimeout: 5 * time.Second}

	require.NoError(t, d.unmount(okMounter{}, "/p"))
	require.Error(t, d.unmount(errMounter{}, "/p"))
}
