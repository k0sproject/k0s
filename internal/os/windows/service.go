//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"sync"

	"golang.org/x/sys/windows/svc"
)

var isService = sync.OnceValues(svc.IsWindowsService)

func IsService() (bool, error) {
	return isService()
}
