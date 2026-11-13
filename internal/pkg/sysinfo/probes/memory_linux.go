// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"sync"

	"github.com/k0sproject/k0s/internal/os/linux"
)

func newTotalMemoryProber() totalMemoryProber {
	return sync.OnceValues(linux.TotalMemory)
}
