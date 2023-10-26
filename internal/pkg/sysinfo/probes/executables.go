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
	"os/exec"
)

func AssertExecutableInPath(p Probes, executable string) {
	p.Set(fmt.Sprintf("executableInPath:%s", executable), func(path ProbePath, _ Probe) Probe {
		return ProbeFn(func(r Reporter) error {
			desc := NewProbeDesc(fmt.Sprintf("Executable in PATH: %s", executable), path)
			path, err := exec.LookPath(executable)
			if err != nil {
				return r.Warn(desc, ErrorProp(err), "")
			}

			return r.Pass(desc, StringProp(path))
		})
	})
}
