// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"errors"
	"fmt"
	"syscall"
)

// openPID can only check the existence of a PID on macOS.
func openPID(pid int) (procHandle, error) {
	// Send "the null signal" to probe if the PID still exists.
	if err := syscall.Kill(pid, syscall.Signal(0)); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("%w on Darwin", errors.ErrUnsupported)
}
