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
		description = fmt.Sprintf("Relative disk space available for %s", a.fsPath)
	} else {
		description = fmt.Sprintf("Disk space available for %s", a.fsPath)
	}
	return NewProbeDesc(description, a.path)
}
