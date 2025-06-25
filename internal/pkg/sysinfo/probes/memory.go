// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
