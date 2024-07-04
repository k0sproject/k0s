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
	"fmt"
	"net"
	"strings"

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
	reporter := &preFlightReporter{log: logrus.NewEntry(logrus.StandardLogger()), lenient: lenient}
	if err := s.NewSysinfoProbes().Probe(reporter); err != nil {
		return fmt.Errorf("pre-flight checks failed, check out `k0s sysinfo`: %w", err)
	}

	if reporter.failed {
		return errors.New("pre-flight checks failed, check out `k0s sysinfo`")
	}

	return nil
}

func (s *K0sSysinfoSpec) NewSysinfoProbes() probes.Probes {
	p := probes.NewRootProbes()

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
	probes.AssertFileSystem(p, s.DataDir)
	probes.AssertFreeDiskSpace(p, s.DataDir, minFreeDiskSpace)
	if s.WorkerRoleEnabled {
		// https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#hard-eviction-thresholds
		probes.AssertRelativeFreeDiskSpace(p, s.DataDir, 15)
	}
	probes.RequireNameResolution(p, net.LookupIP, "localhost")

	s.addHostSpecificProbes(p)

	return p
}

type preFlightReporter struct {
	log             *logrus.Entry
	lenient, failed bool
}

func (p *preFlightReporter) Pass(d probes.ProbeDesc, prop probes.ProbedProp) error {
	if p.log.Logger.IsLevelEnabled(logrus.DebugLevel) {
		p.logger(d, prop).Debug("")
	}
	return nil
}

func (p *preFlightReporter) Warn(d probes.ProbeDesc, prop probes.ProbedProp, msg string) error {
	p.logger(d, prop).Warn(msg)
	return nil
}

func (p *preFlightReporter) Reject(d probes.ProbeDesc, prop probes.ProbedProp, msg string) error {
	p.failed = true

	level := logrus.ErrorLevel
	if p.lenient {
		level = logrus.WarnLevel
	}

	if msg == "" {
		msg = "Rejected"
	} else {
		msg = "Rejected: " + msg
	}

	p.logger(d, prop).Log(level, msg)

	return nil
}

func (p *preFlightReporter) Error(d probes.ProbeDesc, err error) error {
	p.failed = true
	if p.lenient {
		p.logger(d, nil).Error(err)
		return nil
	}

	return err
}

func (p *preFlightReporter) logger(desc probes.ProbeDesc, prop probes.ProbedProp) *logrus.Entry {
	log := p.log.WithField("pre-flight-check", strings.Join(desc.Path(), "/"))
	if prop != nil {
		log = log.WithField("property", prop.String())
	}
	return log
}
