//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package apparmor

import (
	"bytes"
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

// Reports whether the kernel restricts what unprivileged unconfined tasks may
// do. The second return value is false if the kernel doesn't support the
// restriction.
//
// See: https://gitlab.com/apparmor/apparmor/-/wikis/unprivileged_unconfined_restriction#checking-the-current-state-of-restricted-unprivileged-unconfined-restrictions
func UnprivilegedUnconfinedRestricted() (restricted, supported bool, err error) {
	value, err := os.ReadFile("/proc/sys/kernel/apparmor_restrict_unprivileged_unconfined")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, false, nil
		}
		return false, false, err
	}
	return string(bytes.TrimSpace(value)) == "1", true, nil
}
