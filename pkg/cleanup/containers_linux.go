//go:build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/mount-utils"
)

func cleanupContainerMounts() error {
	return removeMount("run/netns")
}

func removeMount(path string) error {
	var errs []error

	mounter := mount.New("")
	procMounts, err := mounter.List()
	if err != nil {
		return err
	}
	for _, v := range procMounts {
		if strings.Contains(v.Path, path) {
			logrus.Debugf("Unmounting: %s", v.Path)
			if err = mounter.Unmount(v.Path); err != nil {
				errs = append(errs, err)
			}

			logrus.Debugf("Removing: %s", v.Path)
			if err := os.RemoveAll(v.Path); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
