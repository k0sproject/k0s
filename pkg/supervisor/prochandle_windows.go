// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"syscall"
)

// newProcHandle is not implemented on Windows.
func newProcHandle(int) (procHandle, error) {
	return nil, syscall.EWINDOWS
}
