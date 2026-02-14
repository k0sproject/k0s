//go:build linux

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"fmt"
	"sync"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

func (c *CgroupsProbes) RequireControllers(controllerNames ...string) {
	c.probeControllers(true, controllerNames...)
}

func (c *CgroupsProbes) AssertControllers(controllerNames ...string) {
	c.probeControllers(false, controllerNames...)
}

func (c *CgroupsProbes) probeControllers(require bool, controllerNames ...string) {
	for _, controllerName := range controllerNames {
		c.Set(controllerName, func(path probes.ProbePath, _ probes.Probe) probes.Probe {
			return &cgroupControllerProbe{
				path,
				c.probeCgroupSystem,
				controllerName,
				require,
			}
		})
	}
}

type cgroupControllerProbe struct {
	path        probes.ProbePath
	probeSystem cgroupSystemProber
	name        string
	require     bool
}

func (c *cgroupControllerProbe) Probe(reporter probes.Reporter) error {
	desc := probes.NewProbeDesc(fmt.Sprintf("cgroup controller %q", c.name), c.path)
	//revive:disable:indent-error-flow
	if sys, err := c.probeSystem(); err != nil {
		return reportCgroupSystemErr(reporter, desc, err)
	} else if available, err := sys.probeController(c.name); err != nil {
		return reporter.Error(desc, err)
	} else if available.available {
		if available.warning != "" {
			return reporter.Warn(desc, available, available.warning)
		}
		return reporter.Pass(desc, available)
	} else if c.require {
		return reporter.Reject(desc, available, "")
	} else {
		return reporter.Warn(desc, available, "")
	}
}

type cgroupControllerAvailable struct {
	available bool
	msg       string
	warning   string
}

func (a cgroupControllerAvailable) String() (msg string) {
	if a.available {
		if a.warning != "" {
			return a.msg
		}

		msg = "available"
	} else {
		msg = "unavailable"
	}

	if a.msg != "" {
		msg = fmt.Sprintf("%s (%s)", msg, a.msg)
	}

	return
}

type cgroupControllerProber struct {
	once        sync.Once
	controllers map[string]cgroupControllerAvailable
	err         error
}

func (p *cgroupControllerProber) probeController(s cgroupSystem, controllerName string) (cgroupControllerAvailable, error) {
	p.once.Do(func() {
		p.controllers = make(map[string]cgroupControllerAvailable)
		p.err = s.loadControllers(func(name, msg string) {
			p.controllers[name] = cgroupControllerAvailable{true, msg, ""}
		})
	})
	return p.controllers[controllerName], p.err
}
