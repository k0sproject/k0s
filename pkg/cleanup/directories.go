//go:build linux

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/mount-utils"
)

// defaultUnmountTimeout bounds a single blocking unmount. A umount(2) of a
// mount whose backend is gone or frozen blocks uninterruptibly and never
// returns, so the lazy fallback is never reached. Bound it so reset makes
// progress.
const defaultUnmountTimeout = 30 * time.Second

// Run removes all kubelet mounts and deletes generated dataDir and runDir
func (d *directories) Run() error {
	// unmount any leftover overlays (such as in alpine)
	mounter := mount.New("")
	procMounts, err := mounter.List()
	if err != nil {
		return err
	}

	var dataDirMounted bool

	// ensure that we don't delete any persistent data volumes that may be
	// mounted by kubernetes by unmount every mount point under DataDir.
	//
	// Unmount in the reverse order it was mounted so we handle recursive
	// bind mounts and over mounts properly. If we for any reason are not
	// able to unmount, fall back to lazy unmount and if that also fails
	// bail out and don't delete anything.
	//
	// Note that if there are any shared bind mounts under k0s data
	// directory, we may end up unmounting stuff outside the k0s DataDir.
	// If someone has set a bind mount to be shared, we assume that is the
	// desired behavior. See MS_SHARED and NOTES:
	//  - https://man7.org/linux/man-pages/man2/mount.2.html
	//  - https://man7.org/linux/man-pages/man2/umount.2.html#NOTES
	for i := len(procMounts) - 1; i >= 0; i-- {
		v := procMounts[i]
		// avoid unmount datadir if its mounted on separate partition
		// k0s didn't mount it so leave it alone
		if v.Path == d.dataDir {
			dataDirMounted = true
			continue
		}
		if isUnderPath(v.Path, d.kubeletRootDir) || isUnderPath(v.Path, d.dataDir) {
			logrus.Debugf("%v is mounted! attempting to unmount...", v.Path)
			if err = d.unmount(mounter, v.Path); err != nil {
				// clean unmount failed or wedged. Detach lazily, which returns
				// at once even on a dead or frozen backend and disconnects the
				// mount from the path so RemoveAll cannot reach volume data.
				logrus.Warningf("lazy unmounting %v: %v", v.Path, err)
				if err = UnmountLazy(v.Path); err != nil {
					return fmt.Errorf("failed unmount %v", v.Path)
				}
			}
		}
	}

	logrus.Debugf("removing kubelet root dir (%s)", d.kubeletRootDir)
	if err := os.RemoveAll(d.kubeletRootDir); err != nil {
		return fmt.Errorf("failed to delete k0s kubelet root direcotory: %w", err)
	}

	if dataDirMounted {
		logrus.Debugf("removing the contents of mounted data-dir (%s)", d.dataDir)
	} else {
		logrus.Debugf("removing k0s generated data-dir (%s)", d.dataDir)
	}

	if err := os.RemoveAll(d.dataDir); err != nil {
		if !dataDirMounted {
			return fmt.Errorf("failed to delete k0s generated data-dir: %w", err)
		}
		if !errorIsUnlinkat(err, d.dataDir) {
			return fmt.Errorf("failed to delete contents of mounted data-dir: %w", err)
		}
	}

	logrus.Debugf("deleting k0s generated run-dir (%s)", d.runDir)
	if err := os.RemoveAll(d.runDir); err != nil {
		return fmt.Errorf("failed to delete %s: %w", d.runDir, err)
	}

	return nil
}

// unmount attempts a normal unmount but never blocks past the timeout. A
// umount(2) can wedge in D state on a dead or frozen backend, so the attempt
// runs in a goroutine we abandon on timeout (the process exits soon after
// reset). A non nil return tells the caller to fall back to a lazy detach.
func (d *directories) unmount(mounter mount.Interface, path string) error {
	timeout := d.unmountTimeout
	if timeout <= 0 {
		timeout = defaultUnmountTimeout
	}
	done := make(chan error, 1)
	go func() { done <- mounter.Unmount(path) }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timed out after %s", timeout)
	}
}

// test if the path is a directory equal to or under base
func isUnderPath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// this is for checking if the error returned by os.RemoveAll is due to
// it being a mount point. if it is, we can ignore the error. this way
// we can't rely on os.RemoveAll instead of recursively deleting the
// contents of the directory
func errorIsUnlinkat(err error, dir string) bool {
	if err == nil {
		return false
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	return pathErr.Path == dir && pathErr.Op == "unlinkat"
}
