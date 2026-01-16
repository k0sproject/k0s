//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"os"
)

// Run removes the k0s data, kubelet root, and run directories.
func (d *directories) Run() error {
	paths := dedupePaths([]string{d.kubeletRootDir, d.dataDir, d.runDir})
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.RemoveAll(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to delete %s: %w", path, err)
		}
	}
	return nil
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var result []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		result = append(result, p)
	}
	return result
}
