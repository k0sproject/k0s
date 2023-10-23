/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
				totalMemory = uint64(info.Totalram) * uint64(info.Unit) // explicit cast to support 32-bit systems
			}
		})

		return totalMemory, err
	}
}
