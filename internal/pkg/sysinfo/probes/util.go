// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"fmt"
)

type probeUnsupported string

func (p probeUnsupported) Error() string {
	return string(p)
}

func (p probeUnsupported) String() string {
	return string(p)
}

const (
	_ = 1 << (10 * iota)
	Ki
	Mi
	Gi
	Ti
)

type iecBytes uint64

func (b iecBytes) String() string {
	const prefixes = "KMGTPE"
	const unit = 1 << 10

	for i := 0; ; i++ {
		x := float32(b) / float32((uint64(1) << (i * 10)))
		if x < unit {
			if i == 0 {
				return fmt.Sprintf("%d B", b)
			}

			return fmt.Sprintf("%.1f %ciB", x, prefixes[i-1])
		}
	}
}
