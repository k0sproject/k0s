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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/containerd/cgroups/v3/cgroup2"
)

type cgroupV2 struct {
	mountPoint  string
	controllers cgroupControllerProber
	probeUname  unameProber
}

func (*cgroupV2) String() string {
	return "version 2"
}

func (g *cgroupV2) probeController(controllerName string) (cgroupControllerAvailable, error) {
	switch controllerName {
	case "devices":
		return g.detectDevicesController()
	case "freezer":
		return g.detectFreezerController()
	}
	return g.controllers.probeController(g, controllerName)
}

func (g *cgroupV2) loadControllers(seen func(string, string)) error {
	return g.detectListedRootControllers(seen)
}

// The device controller has no interface files. Its availability is assumed
// based on the kernel version, as it is hard to detect it directly.
// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#device-controller
func (g *cgroupV2) detectDevicesController() (cgroupControllerAvailable, error) {
	major, minor, err := parseKernelRelease(g.probeUname)
	if err != nil {
		return cgroupControllerAvailable{}, err
	}

	// since 4.15
	available, op := false, "<"
	if major > 4 || (major == 4 && minor >= 15) {
		available, op = true, ">="
	}
	msg := fmt.Sprintf("kernel %d.%d %s 4.15", major, minor, op)
	return cgroupControllerAvailable{available, msg, ""}, nil
}

// Detect the freezer controller. It doesn't appear in the cgroup.controllers
// file. Check for the existence of the cgroup.freeze file in the k0s cgroup
// instead, or try to create a dummy cgroup if k0s runs in the root cgroup.
//
// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#core-interface-files
func (g *cgroupV2) detectFreezerController() (cgroupControllerAvailable, error) {

	// Detect the freezer controller by checking k0s's cgroup for the existence
	// of the cgroup.freeze file.
	// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#processes
	cgroupPath, err := cgroup2.NestedGroupPath("")
	if err != nil {
		return cgroupControllerAvailable{}, fmt.Errorf("failed to get k0s cgroup: %w", err)
	}

	if cgroupPath != "/" {
		cgroupPath = filepath.Join(g.mountPoint, cgroupPath)
	} else { // The root cgroup cannot be frozen. Try to create a dummy cgroup.
		tmpCgroupPath, err := os.MkdirTemp(g.mountPoint, "k0s-freezer-detection-*")
		if err != nil {
			if errors.Is(err, os.ErrPermission) && os.Geteuid() != 0 {
				return cgroupControllerAvailable{true, "unknown", "insufficient permissions, try with elevated permissions"}, nil
			}
			if errors.Is(err, unix.EROFS) && os.Geteuid() != 0 {
				return cgroupControllerAvailable{true, "unknown", fmt.Sprintf("read-only file system: %s", g.mountPoint)}, nil
			}

			return cgroupControllerAvailable{}, fmt.Errorf("failed to create temporary cgroup: %w", err)
		}
		defer func() { err = errors.Join(err, os.Remove(tmpCgroupPath)) }()
		cgroupPath = tmpCgroupPath
	}

	// Check if the cgroup.freeze exists
	if stat, err := os.Stat(filepath.Join(cgroupPath, "cgroup.freeze")); (err == nil && stat.IsDir()) || os.IsNotExist(err) {
		return cgroupControllerAvailable{false, "cgroup.freeze doesn't exist", ""}, nil
	} else if err != nil {
		return cgroupControllerAvailable{}, err
	}
	return cgroupControllerAvailable{true, "cgroup.freeze exists", ""}, nil
}

// Detects all the listed root controllers.
//
// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#core-interface-files
func (g *cgroupV2) detectListedRootControllers(seen func(string, string)) (err error) {
	root, err := cgroup2.Load("/", cgroup2.WithMountpoint(g.mountPoint))
	if err != nil {
		return fmt.Errorf("failed to load root cgroup: %w", err)
	}

	controllerNames, err := root.RootControllers() // This reads cgroup.controllers
	if err != nil {
		return fmt.Errorf("failed to list cgroup root controllers: %w", err)
	}

	for _, controllerName := range controllerNames {
		seen(controllerName, "is a listed root controller")
		switch controllerName {
		case "cpu": // This is the successor to the version 1 cpu and cpuacct controllers.
			seen("cpuacct", "via cpu in "+g.String())
		case "io": // This is the successor of the version 1 blkio controller.
			seen("blkio", "via io in "+g.String())
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
