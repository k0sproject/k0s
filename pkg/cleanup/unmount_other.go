//go:build !linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"runtime"
)

func UnmountLazy(string) error {
	return fmt.Errorf("%w on %s", errors.ErrUnsupported, runtime.GOOS)
}
