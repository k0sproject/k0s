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

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"

	"github.com/sirupsen/logrus"
)

type K0sSysinfoSpec struct {
	ControllerRoleEnabled bool
	WorkerRoleEnabled     bool
	DataDir               string

	// This is mainly for the sysinfo CLI subcommand.
	AddDebugProbes bool

	// May be extended with more flags in the future, e.g. for
	// kube-router, calico, konnectivity, ...
}

func (s *K0sSysinfoSpec) RunPreFlightChecks(lenient bool) error {
	reporter := &preFlightReporter{lenient: lenient}
	if err := s.NewSysinfoProbes().Probe(reporter); err != nil {
		return err
	}

	if reporter.failed {
		return errors.New("pre-flight checks failed")
	}

	return nil
}

func (s *K0sSysinfoSpec) NewSysinfoProbes() probes.Probes {
	p := probes.NewProbes()

	// https://docs.k0sproject.io/main/external-runtime-deps/#a-unique-machine-id-for-multi-node-setups
	probes.RequireMachineID(p)

	// https://docs.k0sproject.io/main/system-requirements/#minimum-memory-and-cpu-requirements
	if s.ControllerRoleEnabled {
		probes.AssertTotalMemory(p, 1*probes.Gi)
	} else if s.WorkerRoleEnabled {
		probes.AssertTotalMemory(p, 500*probes.Mi)
	}

	// https://docs.k0sproject.io/main/system-requirements/#storage
	var minFreeDiskSpace uint64
	if s.ControllerRoleEnabled {
		minFreeDiskSpace = minFreeDiskSpace + 500*probes.Mi
	}
	if s.WorkerRoleEnabled {
		minFreeDiskSpace = minFreeDiskSpace + 1300*probes.Mi
	}
	probes.AssertFreeDiskSpace(p, s.DataDir, minFreeDiskSpace)

	s.addHostSpecificProbes(p)

	return p
}

type preFlightReporter struct {
	lenient, failed bool
}

func (*preFlightReporter) Pass(d probes.ProbeDesc, prop probes.ProbedProp) error {
	logrus.Debug(d.DisplayName(), prop)
	return nil
}

func (*preFlightReporter) Warn(d probes.ProbeDesc, prop probes.ProbedProp, msg string) error {
	logrus.Warn(d.DisplayName(), prop, msg)
	return nil
}

func (p *preFlightReporter) Reject(d probes.ProbeDesc, prop probes.ProbedProp, msg string) error {
	if p.lenient {
		logrus.Warn(d.DisplayName(), prop, msg)
	} else {
		p.failed = true
		logrus.Error(d.DisplayName(), prop, msg)
	}

	return nil
}

func (p *preFlightReporter) Error(d probes.ProbeDesc, err error) error {
	p.failed = true
	return err
}
