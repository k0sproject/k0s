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
	return "", fmt.Errorf("getDefaultNIC on %s is not supported", runtime.GOOS)
}

func macToInterfaceName(_ *string, errs *[]error) {
	*errs = append(*errs, fmt.Errorf("%w on %s: resolving interface names for MAC addresses", errors.ErrUnsupported, runtime.GOOS))
}
