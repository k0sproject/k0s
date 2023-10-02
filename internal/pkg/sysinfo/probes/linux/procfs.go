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
	"os"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

func (l *LinuxProbes) RequireProcFS() {
	l.Set("procfs", func(path probes.ProbePath, _ probes.Probe) probes.Probe {
		return probes.ProbeFn(func(r probes.Reporter) error {
			mountPoint := "/proc"
			desc := probes.NewProbeDesc(fmt.Sprintf("%s file system", mountPoint), path)

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
