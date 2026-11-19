//go:build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package uninstall

import (
	"errors"
	"os"
)

func ensurePrivileges() error {
	if os.Geteuid() != 0 {
		return errors.New("this command must be run as root")
	}
	return nil
}
