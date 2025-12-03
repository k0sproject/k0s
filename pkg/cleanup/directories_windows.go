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
)

// Run removes the k0s data, kubelet root, and run directories.
func (d *directories) Run() error {
	var errs []error
	paths := dedupePaths([]string{d.kubeletRootDir, d.dataDir, d.runDir})
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.RemoveAll(path); err != nil && !errors.Is(err, os.ErrNotExist) {
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
