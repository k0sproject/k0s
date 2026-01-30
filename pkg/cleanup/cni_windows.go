//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Run removes Windows CNI artifacts.
func (c *cni) Run() error {
	var errs []error

	files := []string{
		`C:\etc\cni\net.d\10-calico.conflist`,
		`C:\etc\cni\net.d\calico-kubeconfig`,
		`C:\etc\cni\net.d\10-kuberouter.conflist`,
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("failed to remove %s: %w", filepath.Clean(f), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while removing CNI leftovers: %w", errors.Join(errs...))
	}
	return nil
}
