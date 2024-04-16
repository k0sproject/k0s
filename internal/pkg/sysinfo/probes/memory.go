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
	"errors"
	"fmt"
)

// AssertTotalMemory asserts a minimum amount of system RAM.
func AssertTotalMemory(parent ParentProbe, min uint64) {
	parent.Set("memory", func(path ProbePath, current Probe) Probe {
		if p, ok := current.(*assertTotalMem); ok {
			p.minFree = min
			return p
		}

		return &assertTotalMem{path, newTotalMemoryProber(), min}
	})
}

type assertTotalMem struct {
	path             ProbePath
	probeTotalMemory totalMemoryProber
	minFree          uint64
}

func (a *assertTotalMem) Probe(reporter Reporter) error {
	desc := NewProbeDesc("Total memory", a.path)
	if totalMemory, err := a.probeTotalMemory(); err != nil {
		var unsupportedErr probeUnsupported
		if errors.As(err, &unsupportedErr) {
			return reporter.Warn(desc, unsupportedErr, "")
		}
		return reporter.Error(desc, err)
	} else if totalMemory >= a.minFree {
		return reporter.Pass(desc, iecBytes(totalMemory))
	} else {
		return reporter.Warn(desc, iecBytes(totalMemory), fmt.Sprintf("%s recommended", iecBytes(a.minFree)))
	}
}

type totalMemoryProber func() (uint64, error)
