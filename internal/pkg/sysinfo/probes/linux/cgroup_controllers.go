//go:build linux
// +build linux

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
	if sys, err := c.probeSystem(); err != nil {
		return reportCgroupSystemErr(reporter, desc, err)
	} else if available, err := sys.probeController(c.name); err != nil {
		return reporter.Error(desc, err)
	} else if available.available {
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
}

func (a cgroupControllerAvailable) String() (msg string) {
	if a.available {
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

func (p *cgroupControllerProber) probeContoller(s cgroupSystem, controllerName string) (cgroupControllerAvailable, error) {
	p.once.Do(func() {
		p.controllers = make(map[string]cgroupControllerAvailable)
		p.err = s.loadControllers(func(name, msg string) {
			p.controllers[name] = cgroupControllerAvailable{true, msg}
		})
	})
	return p.controllers[controllerName], p.err
}
