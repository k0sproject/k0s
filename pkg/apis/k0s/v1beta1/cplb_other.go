//go:build !linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"fmt"
	"runtime"
)

func getDefaultNIC() (string, error) {
	return "", fmt.Errorf("%w on %s", errors.ErrUnsupported, runtime.GOOS)
}

func getNIC(string) (string, error) {
	return "", fmt.Errorf("%w on %s", errors.ErrUnsupported, runtime.GOOS)
}
