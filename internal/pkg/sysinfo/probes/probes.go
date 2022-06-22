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

// Package probes provides a framework for implementing and executing "probes".
// A probe represents some check of a certain property, that may simply pass,
// warn about that property, or reject it.
//
// The design of the probes package is inspired by the way Go tests are written.
// Probes are run via their Probe method that receives a Reporter (think
// *testing.T) as argument. Probes may or may not contain nested probes. They
// form a hierarchy. Probes are typically added to a Probes object by functions
// that either start with "Require", in which they reject properties that don't
// pass, or with "Assert", in which they just warn about them.
package probes

import "reflect"

// Probe represents some check that yields its outcome to a Reporter.
type Probe interface {
	// Probe executes this probe, reporting its outcome to the given Reporter.
	// The returned error is typically forwarded from the invocation of a
	// Reporter method.
	Probe(Reporter) error
}

type ProbeFn func(Reporter) error

func (fn ProbeFn) Probe(r Reporter) error {
	return fn(r)
}

// ProbePath identifies a probe in its hierarchy.
type ProbePath []string

func (p ProbePath) Equal(other ProbePath) bool {
	return reflect.DeepEqual(p, other)
}

// ProbeDesc describes a probe.
type ProbeDesc interface {
	// Path of a probe, which identifies it in a machine readable way.
	Path() ProbePath

	// DisplayName returns a human readable name for a probe.
	DisplayName() string
}

// NewProbeDesc returns a new ProbeDesc with the given display name and path.
func NewProbeDesc(name string, path ProbePath) ProbeDesc {
	return &probeDesc{path, name}
}

type ParentProbe interface {
	Get(id string) Probe
	Set(id string, setter func(path ProbePath, current Probe) Probe)
}

// ProbedProp represents the property that has been inspected by a probe.
type ProbedProp interface {
	// Name returns the string representation of this property.
	String() string
}

// StringProp is a convenience way of reporting an arbitrary string as a probed property.
type StringProp string

func (s StringProp) String() string {
	return string(s)
}

// ErrorProp is a convenience way of reporting an arbitrary error as a probed property.
func ErrorProp(err error) interface {
	ProbedProp
	error
} {
	return errorProp{err}
}

// Reporter receives the outcome of probes.
type Reporter interface {
	// Pass informs about a probe that passed.
	Pass(ProbeDesc, ProbedProp) error

	// Warn informs about a probe that passed, but produced some sort of
	// warning.
	Warn(d ProbeDesc, prop ProbedProp, msg string) error

	// Reject informs about a probe that rejected its value.
	Reject(d ProbeDesc, prop ProbedProp, msg string) error

	// Error informs about some error that prevented the probe from producing a
	// meaningful result.
	Error(ProbeDesc, error) error
}

// Probes represents a "composite" Probe.
type Probes interface {
	ParentProbe
	Probe
}

// NewRootProbes returns a new, empty composite probe without a path.
func NewRootProbes() Probes {
	return &probes{nil, nil}
}

// NewProbesAtPath returns a new, empty composite probe at the given path.
func NewProbesAtPath(path ProbePath) Probes {
	return &probes{path, nil}
}

type probeDesc struct {
	path ProbePath
	name string
}

func (d *probeDesc) Path() ProbePath     { return d.path }
func (d *probeDesc) DisplayName() string { return d.name }

type errorProp struct{ error }

func (e errorProp) String() string {
	return e.Error()
}

func (e errorProp) Unwrap() error {
	return e.error
}

type probes struct {
	path   ProbePath
	probes []*containedProbe
}

type containedProbe struct {
	id    string
	probe Probe
}

func (p *probes) Get(id string) Probe {
	for _, probe := range p.probes {
		if probe.id == id {
			return probe.probe
		}
	}

	return nil
}

func (p *probes) Set(id string, setter func(ProbePath, Probe) Probe) {
	path := append(p.path, id)
	for _, probe := range p.probes {
		if probe.id == id {
			probe.probe = ensureSet(setter(path, probe.probe))
			return
		}
	}

	p.probes = append(p.probes, &containedProbe{id, ensureSet(setter(path, nil))})
}

func ensureSet(probe Probe) Probe {
	if probe == nil {
		panic("probe not set")
	}

	return probe
}

// Probe executes all contained probes, short-circuiting as soon as the first
// one reports an error.
func (p *probes) Probe(reporter Reporter) error {
	for _, probe := range p.probes {
		if err := probe.probe.Probe(reporter); err != nil {
			return err
		}
	}

	return nil
}
