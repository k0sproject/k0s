//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

// Run removes the k0s data, kubelet root, and run directories.
func (d *directories) Run() error {
	var errs []error
	paths := dedupePaths([]string{d.kubeletRootDir, d.dataDir, d.runDir})
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := removeDirectory(path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func removeDirectory(path string) error {
	err := os.RemoveAll(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	// Deletion failed, try taking ownership and resetting permissions.
	logrus.Debugf("initial deletion of %s failed, attempting to take ownership", path)
	takeOwnership(path)

	err = retry.Do(
		func() error {
			return os.RemoveAll(path)
		},
		retry.Attempts(5),
		retry.Delay(2*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			return err != nil && !errors.Is(err, os.ErrNotExist)
		}),
		retry.OnRetry(func(n uint, err error) {
			logrus.WithError(err).Debugf("retrying deletion of %s (attempt %d)", path, n+1)
		}),
	)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete %s: %w", path, err)
	}
	return nil
}

// takeOwnership uses takeown and icacls to take ownership and grant full
// control to administrators, handling containerd snapshots with restrictive ACLs.
func takeOwnership(path string) {
	if out, err := exec.Command("takeown", "/F", path, "/R", "/A", "/D", "Y").CombinedOutput(); err != nil {
		logrus.WithError(err).Debugf("takeown failed for %s: %s", path, string(out))
	} else {
		logrus.Debugf("took ownership of %s", path)
	}

	if out, err := exec.Command("icacls", path, "/grant", "administrators:F", "/T", "/C", "/Q").CombinedOutput(); err != nil {
		logrus.WithError(err).Debugf("icacls failed for %s: %s", path, string(out))
	} else {
		logrus.Debugf("granted permissions on %s", path)
	}
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var result []string
	for _, p := range paths {
		cleaned := filepath.Clean(p)
		if cleaned == "." || cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}
