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
	"runtime"

	system "k8s.io/system-validators/validators"
)

type cgroupVersion int

const (
	cgroupVersionUnknown cgroupVersion = -1
	cgroupV1             cgroupVersion = 1
	cgroupV2             cgroupVersion = 2
)

const (
	/* for some reason, those values aren't public in the system-validators */

	good system.ValidationResultType = 0
	bad  system.ValidationResultType = 1
)

// K0sSpec defines the requirements of systems supported by k0s.
type K0sSpec struct {
	// General requirements implemented by system-validators
	sys system.SysSpec

	// Supported cgroups versions (skipped if nil/empty)
	supportedCgroupVersions []cgroupVersion
}

type noopReporter struct{}

func (r *noopReporter) Report(string, string, system.ValidationResultType) error {
	return nil
}

func (s *K0sSpec) validate() (errs []error) {
	return s.run(&noopReporter{})
}

func (s *K0sSpec) run(reporter system.Reporter) (errs []error) {
	if runtime.GOOS == "linux" {
		_, validationErrs := (&system.OSValidator{Reporter: reporter}).Validate(s.sys)
		errs = append(errs, validationErrs...)
	}

	_, validationErrs := (&system.KernelValidator{Reporter: reporter}).Validate(s.sys)
	errs = append(errs, validationErrs...)

	if runtime.GOOS == "linux" {
		if err := s.validateCgroupVersion(reporter); err != nil {
			errs = append(errs, err)
		} else {
			_, validationErrs := (&system.CgroupsValidator{Reporter: reporter}).Validate(s.sys)
			errs = append(errs, validationErrs...)
		}
	}

	return errs
}
