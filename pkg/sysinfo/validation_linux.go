//go:build linux
// +build linux

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

package sysinfo

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
	system "k8s.io/system-validators/validators"
)

const cgroupMountpoint = "/sys/fs/cgroup"

func getCgroupVersion() (cgroupVersion, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(cgroupMountpoint, &st); err != nil {
		return cgroupVersionUnknown, fmt.Errorf("failed to stat %q: %w", cgroupMountpoint, err)
	}

	switch st.Type {
	case unix.CGROUP2_SUPER_MAGIC:
		return cgroupV2, nil
	case unix.CGROUP_SUPER_MAGIC, unix.TMPFS_MAGIC:
		return cgroupV1, nil
	default:
		return cgroupVersionUnknown, fmt.Errorf("unexpected file system type of %q: 0x%x", cgroupMountpoint, st.Type)
	}
}

func (s *K0sSpec) validateCgroupVersion(reporter system.Reporter) error {
	if len(s.supportedCgroupVersions) < 1 {
		return nil
	}

	actualVersion, err := getCgroupVersion()
	if err != nil {
		return fmt.Errorf("failed to determine cgroup version: %w", err)
	}

	for _, supportedVersion := range s.supportedCgroupVersions {
		if actualVersion == supportedVersion {
			//nolint:errcheck // the returned err seems to be always ignored in system-validators
			reporter.Report("CGROUP_VERSION", fmt.Sprintf("v%d", actualVersion), good)
			return nil
		}
	}

	//nolint:errcheck // the returned err seems to be always ignored in system-validators
	reporter.Report("CGROUP_VERSION", fmt.Sprintf("v%d", actualVersion), bad)
	return fmt.Errorf("unsupported cgroup version: v%d", actualVersion)
}
