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

import "github.com/k0sproject/k0s/internal/pkg/sysinfo/machineid"

// RequireMachineID requires a Machine ID
func RequireMachineID(parent ParentProbe) {
	parent.Set("machine-id", func(path ProbePath, _ Probe) Probe {
		return ProbeFn(func(r Reporter) error {
			desc := NewProbeDesc("Machine ID", path)
			machineID, err := machineid.Generate()
			if err != nil {
				return r.Error(desc, err)
			}
			return r.Pass(desc, machineID)
		})
	})
}
