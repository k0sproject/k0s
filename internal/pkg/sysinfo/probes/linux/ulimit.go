//go:build linux

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
