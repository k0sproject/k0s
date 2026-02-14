//go:build linux

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

func (l *LinuxProbes) RequireProcFS() {
	l.Set("procfs", func(path probes.ProbePath, _ probes.Probe) probes.Probe {
		return probes.ProbeFn(func(r probes.Reporter) error {
			mountPoint := "/proc"
			desc := probes.NewProbeDesc(mountPoint+" file system", path)

			var st syscall.Statfs_t
			if err := syscall.Statfs(mountPoint, &st); err != nil {
				if os.IsNotExist(err) {
					return r.Reject(desc, probes.ErrorProp(err), "")
				}

				return r.Error(desc, fmt.Errorf("failed to statfs %q: %w", mountPoint, err))
			}

			if st.Type != unix.PROC_SUPER_MAGIC {
				return r.Reject(desc, procFSType(st.Type), "unexpected file system type")
			}

			return r.Pass(desc, procFSType(st.Type))
		})
	})
}

type procFSType int64

func (t procFSType) String() string {
	return fmt.Sprintf("mounted (0x%x)", int64(t))
}
