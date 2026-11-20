// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"fmt"
	"sync"
	"syscall"
)

func newTotalMemoryProber() totalMemoryProber {
	return sync.OnceValues(func() (uint64, error) {
		var info syscall.Sysinfo_t
		if err := syscall.Sysinfo(&info); err != nil {
			return 0, fmt.Errorf("sysinfo syscall failed: %w", err)
		} else {
			//nolint:unconvert // explicit cast to support 32-bit systems
			return uint64(info.Totalram) * uint64(info.Unit), nil
		}
	})
}
