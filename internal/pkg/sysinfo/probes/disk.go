// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

// AssertDiskSpace asserts a minimum amount of free disk space.
func AssertFreeDiskSpace(parent ParentProbe, fsPath string, minFree uint64) {
	parent.Set("disk:"+fsPath, func(path ProbePath, current Probe) Probe {
		return &assertDiskSpace{path, fsPath, minFree, false}
	})
}

// AssertDiskSpace asserts a minimum amount of free disk space.
func AssertRelativeFreeDiskSpace(parent ParentProbe, fsPath string, minFreePercent uint64) {
	parent.Set("reldisk:"+fsPath, func(path ProbePath, current Probe) Probe {
		return &assertDiskSpace{path, fsPath, minFreePercent, true}
	})
}

type assertDiskSpace struct {
	path       ProbePath
	fsPath     string
	minFree    uint64
	isRelative bool
}

func (a *assertDiskSpace) desc() ProbeDesc {
	var description string
	if a.isRelative {
		description = "Relative disk space available for " + a.fsPath
	} else {
		description = "Disk space available for " + a.fsPath
	}
	return NewProbeDesc(description, a.path)
}
