// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysinfo

import (
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	"github.com/stretchr/testify/mock"
)

type MockReporter struct {
	mock.Mock
}

func (m *MockReporter) Pass(d probes.ProbeDesc, prop probes.ProbedProp) error {
	args := m.Called(d, prop)
	return args.Error(0)
}

func (m *MockReporter) Warn(d probes.ProbeDesc, prop probes.ProbedProp, msg string) error {
	args := m.Called(d, prop, msg)
	return args.Error(0)
}

func (m *MockReporter) Reject(d probes.ProbeDesc, prop probes.ProbedProp, msg string) error {
	args := m.Called(d, prop, msg)
	return args.Error(0)
}

func (m *MockReporter) Error(d probes.ProbeDesc, err error) error {
	args := m.Called(d, err)
	return args.Error(0)
}
