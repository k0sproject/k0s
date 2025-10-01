// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

// AssertFileSystem asserts a minimum amount of free disk space.
func AssertFileSystem(parent ParentProbe, fsPath string) {
	parent.Set("filesystem:"+fsPath, func(path ProbePath, current Probe) Probe {
		return &assertFileSystem{path, fsPath}
	})
}

type assertFileSystem struct {
	path   ProbePath
	fsPath string
}

func (a *assertFileSystem) desc() ProbeDesc {
	return NewProbeDesc("File system of "+a.fsPath, a.path)
}
