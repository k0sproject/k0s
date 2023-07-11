//go:build linux

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
	"errors"
	"fmt"
	"path"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	test_sysinfo "github.com/k0sproject/k0s/internal/testutil/sysinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequireCgroups(t *testing.T) {
	path := probes.ProbePath{t.Name()}
	linux := newLinuxProbes(path)

	cgroupsA := linux.RequireCgroups()
	cgroupsB := linux.RequireCgroups()

	assert.Same(t, cgroupsA, cgroupsB)
}

func TestCgroupsProbes_Probe(t *testing.T) {
	path := probes.ProbePath{t.Name()}
	var mockSys *mockCgroupSystem
	var mockSysErr error

	reporter := new(test_sysinfo.MockReporter)

	init := func() {
		reporter.Mock = mock.Mock{}
		mockSys, mockSysErr = new(mockCgroupSystem), nil
	}

	underTest := newCgroupsProbes(path, nil, "")
	underTest.probeCgroupSystem = func() (cgroupSystem, error) { return mockSys, mockSysErr }
	underTest.AssertControllers("foo")

	t.Run("Pass", func(t *testing.T) {
		init()

		available := cgroupControllerAvailable{true, "", ""}

		reporter.On("Pass", mock.Anything, mockSys).Return(nil)
		mockSys.On("probeController", "foo").Return(available, nil)
		reporter.On("Pass", mock.Anything, available).Return(nil)

		err := underTest.Probe(reporter)

		reporter.AssertExpectations(t)
		mockSys.AssertExpectations(t)
		assert.NoError(t, err)
		assert.Equal(t, path, reporter.Calls[0].Arguments[0].(probes.ProbeDesc).Path())
		assert.Equal(t, append(path, "foo"), reporter.Calls[1].Arguments[0].(probes.ProbeDesc).Path())
	})

	t.Run("Error", func(t *testing.T) {
		init()

		mockSys, mockSysErr = nil, errors.New(t.Name())

		reporter.On("Error", mock.Anything, mockSysErr).Return(mockSysErr)

		err := underTest.Probe(reporter)

		reporter.AssertExpectations(t)
		assert.Same(t, mockSysErr, err)
		assert.Len(t, reporter.Calls, 1)
	})

}

func TestCgroupsProbes_Probe_NonExistent(t *testing.T) {
	nonExistent := path.Join(t.TempDir(), "non-existent")
	path := probes.ProbePath{t.Name()}
	reporter := new(test_sysinfo.MockReporter)
	reporter.On("Reject", mock.Anything, mock.Anything, "").Return(nil)

	underTest := newCgroupsProbes(path, nil, nonExistent)
	err := underTest.Probe(reporter)

	assert.NoError(t, err)
	reporter.AssertExpectations(t)
	args := reporter.Calls[0].Arguments
	assert.Equal(t, path, args[0].(probes.ProbeDesc).Path())
	assert.Equal(t, fmt.Sprintf("no file system mounted at %q", nonExistent), args[1].(error).Error())
}

type mockCgroupSystem struct {
	mock.Mock
	cgroupSystem
}

func (m *mockCgroupSystem) probeController(name string) (cgroupControllerAvailable, error) {
	args := m.Called(name)
	return args.Get(0).(cgroupControllerAvailable), args.Error(1)
}

func (m *mockCgroupSystem) String() string {
	return "mockCgroupSystem"
}
