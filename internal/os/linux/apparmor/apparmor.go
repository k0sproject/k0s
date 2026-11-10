//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package apparmor

import (
	"errors"
	"os"
)

const (
	Unavailable = "unavailable"
	Disabled    = "disabled"
	Active      = "active"
)

// Reports the AppArmor status in the running kernel.
func Status() (string, error) {
	enabled, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The file is missing. Make sure that's not just because sysfs isn't mounted.
			if _, err := os.Stat("/sys/module"); err != nil {
				return "", err
			}
			return Unavailable, nil
		}
		return "", err
	}

	if len(enabled) > 0 && enabled[0] == 'Y' {
		return Active, nil
	}

	return Disabled, nil
}
