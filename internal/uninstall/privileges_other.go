//go:build !linux && !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package uninstall

import (
	"errors"
	"fmt"
	"runtime"
)

func ensurePrivileges() error {
	return fmt.Errorf("%w on %s", errors.ErrUnsupported, runtime.GOOS)
}
