// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"os/exec"
)

func AssertExecutableInPath(p Probes, executable string) {
	p.Set("executableInPath:"+executable, func(path ProbePath, _ Probe) Probe {
		return ProbeFn(func(r Reporter) error {
			desc := NewProbeDesc("Executable in PATH: "+executable, path)
			path, err := exec.LookPath(executable)
			if err != nil {
				return r.Warn(desc, ErrorProp(err), "")
			}

			return r.Pass(desc, StringProp(path))
		})
	})
}
