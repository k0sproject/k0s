// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"fmt"
	"sync"
	"syscall"
)

func newTotalMemoryProber() totalMemoryProber {
	var once sync.Once
	var totalMemory uint64
	var err error

	return func() (uint64, error) {
		once.Do(func() {
			var info syscall.Sysinfo_t
			if err = syscall.Sysinfo(&info); err != nil {
				err = fmt.Errorf("sysinfo syscall failed: %w", err)
			} else {
				//nolint:unconvert // explicit cast to support 32-bit systems
				totalMemory = uint64(info.Totalram) * uint64(info.Unit)
			}
		})

		return totalMemory, err
	}
}
