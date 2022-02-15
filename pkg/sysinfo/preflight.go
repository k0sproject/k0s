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

package sysinfo

import (
	"errors"
	"strings"

	system "k8s.io/system-validators/validators"
)

func preflightSpec() K0sSpec {
	spec := K0sSpec{
		sys:                     system.DefaultSysSpec,
		supportedCgroupVersions: []cgroupVersion{cgroupV1, cgroupV2},
	}

	spec.sys.KernelSpec.Optional = nil

	return spec
}

// RunPreFlightChecks performs k0s's preflight checks.
func RunPreFlightChecks() error {
	spec := preflightSpec()
	if preflightErrors := spec.validate(); len(preflightErrors) > 0 {
		var msg strings.Builder

		msg.WriteString("pre-flight checks failed: ")
		msg.WriteString(preflightErrors[0].Error())

		for _, err := range preflightErrors[1:] {
			msg.WriteString("; ")
			msg.WriteString(err.Error())
		}

		return errors.New(msg.String())
	}

	return nil
}
