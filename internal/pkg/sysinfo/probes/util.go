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

	for i := 0; ; i = i + 1 {
		x := float32(b) / float32((uint64(1) << (i * 10)))
		if x < unit {
			if i == 0 {
				return fmt.Sprintf("%d B", b)
			}

			return fmt.Sprintf("%.1f %ciB", x, prefixes[i-1])
		}
	}
}
