//go:build linux

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

package linux

import (
	"fmt"
	"syscall"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

func (l *LinuxProbes) AssertProcessMaxFileDescriptors(min uint64) {
	l.Set("RLIMIT_NOFILE", func(path probes.ProbePath, _ probes.Probe) probes.Probe {
		return probes.ProbeFn(func(r probes.Reporter) error {
			desc := probes.NewProbeDesc("Max. file descriptors per process", path)

			var rlimit syscall.Rlimit
			if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
				return r.Warn(desc, probes.ErrorProp(err), "")
			}

			prop := probes.StringProp(fmt.Sprintf("current: %d / max: %d", rlimit.Cur, rlimit.Max))
			if rlimit.Cur < min {
				return r.Warn(desc, prop, fmt.Sprintf("< %d", min))
			}

			return r.Pass(desc, prop)
		})
	})
}
