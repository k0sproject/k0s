//go:build !unix

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"errors"
	"fmt"
	"runtime"
)

func LookupUID(string) (int, error) {
	return 0, fmt.Errorf("%w on %s", errors.ErrUnsupported, runtime.GOOS)
}
