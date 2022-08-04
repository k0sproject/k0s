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

package linux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type cgroupV2 struct {
	mountPoint  string
	controllers cgroupControllerProber
	probeUname  unameProber
}

func (*cgroupV2) String() string {
	return "version 2"
}

func (s *cgroupV2) probeController(controllerName string) (cgroupControllerAvailable, error) {
	return s.controllers.probeContoller(s, controllerName)
}

func (s *cgroupV2) loadControllers(seen func(string, string)) error {
	// Some controllers are implicitly enabled by the kernel. Those controllers
	// do not appear in /sys/fs/cgroup/cgroup.controllers. Their availability is
	// assumed based on the kernel version, as it is hard to detect them
	// directly.
	// https://github.com/torvalds/linux/blob/v5.3/kernel/cgroup/cgroup.c#L433-L434
	if major, minor, err := parseKernelRelease(s.probeUname); err == nil {
		/* devices: since 4.15 */ if major > 4 || (major == 4 && minor >= 15) {
			seen("devices", "assumed")
		}
		/* freezer: since 5.2 */ if major > 5 || (major == 5 && minor >= 2) {
			seen("freezer", "assumed")
		}
	} else {
		return err
	}

	controllerData, err := os.ReadFile(filepath.Join(s.mountPoint, "cgroup.controllers"))
	if err != nil {
		return err
	}

	for _, controllerName := range strings.Fields(string(controllerData)) {
		seen(controllerName, "")
		switch controllerName {
		case "cpu": // This is the successor to the version 1 cpu and cpuacct controllers.
			seen("cpuacct", "via cpu in "+s.String())
		case "io": // This is the successor of the version 1 blkio controller.
			seen("blkio", "via io in "+s.String())
		}
	}

	return nil
}

func parseKernelRelease(probeUname unameProber) (int64, int64, error) {
	uname, err := probeUname()
	if err != nil {
		return 0, 0, err
	}

	var major, minor int64
	r := regexp.MustCompile(`^(\d+)\.(\d+)(\.|$)`)
	if matches := r.FindStringSubmatch(uname.osRelease.value); matches == nil {
		err = errors.New("unsupported format")
	} else {
		if major, err = strconv.ParseInt(matches[1], 10, 16); err == nil {
			minor, err = strconv.ParseInt(matches[2], 10, 16)
		}
	}

	if err != nil {
		err = fmt.Errorf("failed to parse kernel release %q: %w", uname.osRelease, err)
	}

	return major, minor, err
}
