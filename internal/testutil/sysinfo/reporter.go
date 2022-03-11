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
