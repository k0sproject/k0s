/*
Copyright 2024 k0s authors

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
	return NewProbeDesc(fmt.Sprintf("File system of %s", a.fsPath), a.path)
}
