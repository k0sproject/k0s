//go:build linux

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/sirupsen/logrus"
)

// Run removes found CNI leftovers
func (c *cni) Run() error {
	var errs []error

	files := []string{
		"/etc/cni/net.d/10-calico.conflist",
		"/etc/cni/net.d/calico-kubeconfig",
		"/etc/cni/net.d/10-kuberouter.conflist",
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logrus.Debug("failed to remove", f, err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while removing CNI leftovers: %w", errors.Join(errs...))
	}
	return nil
}
