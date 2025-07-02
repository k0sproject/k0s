//go:build !linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"
	"runtime"
)

func getDefaultNIC() (string, error) {
	return "", fmt.Errorf("getDefaultNIC on %s is not supported", runtime.GOOS)
}
