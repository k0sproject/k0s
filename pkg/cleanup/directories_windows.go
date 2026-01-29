//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

// Run removes the k0s data, kubelet root, and run directories.
// On Windows, files like containerd VHDX snapshots may remain locked briefly
// after processes exit, so we retry with backoff.
func (d *directories) Run() error {
	var errs []error
	paths := dedupePaths([]string{d.kubeletRootDir, d.dataDir, d.runDir})
	for _, path := range paths {
		if path == "" {
			continue
		}
		err := retry.Do(
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
				logrus.WithError(err).Debugf("Retrying deletion of %s (attempt %d)", path, n+1)
			}),
		)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("failed to delete %s: %w", path, err))
		}
	}
	return errors.Join(errs...)
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
