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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/cilium/ebpf/rlimit"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
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

// Detects the device controller by trying to attach a dummy program of type
// BPF_CGROUP_DEVICE to a cgroup. Since the controller has no interface files
// and is implemented purely on top of BPF, this is the only reliable way to
// detect it. A best-guess detection via the kernel version has the major
// drawback of not working with kernels that have a lot of backported features,
// such as RHEL and friends.
//
// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#device-controller
func (g *cgroupV2) detectDevicesController() (cgroupControllerAvailable, error) {
	err := attachDummyDeviceFilter(g.mountPoint)
	switch {
	case err == nil:
		return cgroupControllerAvailable{true, "device filters attachable", ""}, nil

	// EACCES occurs when not allowed to create cgroups.
	// EPERM occurs when not allowed to load eBPF programs.
	case errors.Is(err, os.ErrPermission) && os.Geteuid() != 0:
		return cgroupControllerAvailable{true, "unknown", "insufficient permissions, try with elevated permissions"}, nil
	case errors.Is(err, unix.EROFS):
		return cgroupControllerAvailable{true, "unknown", fmt.Sprintf("read-only file system: %s", g.mountPoint)}, nil

	case eBPFProgramUnsupported(err):
		return cgroupControllerAvailable{false, err.Error(), ""}, nil
	}

	return cgroupControllerAvailable{}, err
}

// Attaches a dummy program of type BPF_CGROUP_DEVICE to a randomly created
// cgroup and removes the program and cgroup again.
func attachDummyDeviceFilter(mountPoint string) (err error) {
	insts, license, err := cgroup2.DeviceFilter([]specs.LinuxDeviceCgroup{{
		Allow:  true,
		Type:   "a",
		Major:  ptr.To(int64(-1)),
		Minor:  ptr.To(int64(-1)),
		Access: "rwm",
	}})
	if err != nil {
		return fmt.Errorf("failed to create eBPF device filter program: %w", err)
	}

	tmpCgroupPath, err := os.MkdirTemp(mountPoint, "k0s-devices-detection-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary cgroup: %w", err)
	}
	defer func() { err = errors.Join(err, os.Remove(tmpCgroupPath)) }()

	dirFD, err := unix.Open(tmpCgroupPath, unix.O_DIRECTORY|unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("failed to open temporary cgroup: %w", &fs.PathError{Op: "open", Path: tmpCgroupPath, Err: err})
	}
	defer func() {
		if closeErr := unix.Close(dirFD); closeErr != nil {
			err = errors.Join(err, &fs.PathError{Op: "close", Path: tmpCgroupPath, Err: closeErr})
		}
	}()

	close, err := cgroup2.LoadAttachCgroupDeviceFilter(insts, license, dirFD)
	if err != nil {
		// RemoveMemlock may be required on kernels < 5.11
		// observed on debian 11: 5.10.0-21-armmp-lpae #1 SMP Debian 5.10.162-1 (2023-01-21) armv7l
		// https://github.com/cilium/ebpf/blob/v0.11.0/prog.go#L356-L360
		if errors.Is(err, unix.EPERM) && strings.Contains(err.Error(), "RemoveMemlock") {
			if err2 := rlimit.RemoveMemlock(); err2 != nil {
				err = errors.Join(err, err2)
			} else {
				// Try again, MEMLOCK should be removed by now.
				close, err2 = cgroup2.LoadAttachCgroupDeviceFilter(insts, license, dirFD)
				if err2 != nil {
					err = errors.Join(err, err2)
				} else {
					err = nil
				}
			}
		}
	}
	if err != nil {
		if eBPFProgramUnsupported(err) {
			return err
		}
		return fmt.Errorf("failed to load/attach eBPF device filter program: %w", err)
	}

	return close()
}

// Returns true if the given error indicates that an eBPF program is unsupported
// by the kernel.
func eBPFProgramUnsupported(err error) bool {
	// https://github.com/cilium/ebpf/blob/v0.11.0/features/prog.go#L43-L49

	switch {
	// EINVAL occurs when attempting to create a program with an unknown type.
	case errors.Is(err, unix.EINVAL):
		return true

	// E2BIG occurs when ProgLoadAttr contains non-zero bytes past the end of
	// the struct known by the running kernel, meaning the kernel is too old to
	// support the given prog type.
	case errors.Is(err, unix.E2BIG):
		return true

	default:
		return false
	}
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
