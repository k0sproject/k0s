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
	"strings"
	"syscall"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"

	test_sysinfo "github.com/k0sproject/k0s/internal/testutil/sysinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequireLinux(t *testing.T) {

	p := probes.NewRootProbes()
	linuxProbes := RequireLinux(p)

	t.Run("reusesPreviousProbe", func(t *testing.T) {
		assert.Same(t, linuxProbes, RequireLinux(p))
	})

	linux := unameField{"Linux", false}
	minix := unameField{"Minix", false}

	expectedErr := errors.New("dummy")
	expectedPath := func(t *testing.T) interface{} {
		return mock.MatchedBy(func(d probes.ProbeDesc) bool {
			assert.Equal(t, probes.ProbePath{"os"}, d.Path())
			assert.Equal(t, "Operating system", d.DisplayName())
			return true
		})
	}

	var unameResult *uname
	var unameErr error
	linuxProbes.probeUname = func() (*uname, error) { return unameResult, unameErr }

	t.Run("probePasses", func(t *testing.T) {
		unameResult = &uname{osName: linux}
		unameErr = nil
		reporter := new(test_sysinfo.MockReporter)
		reporter.On("Pass", expectedPath(t), linux).Return(expectedErr)

		err := p.Probe(reporter)

		reporter.AssertExpectations(t)
		assert.Same(t, expectedErr, err)
	})

	t.Run("probeRejectsNonLinux", func(t *testing.T) {
		unameResult = &uname{osName: minix}
		unameErr = nil

		reporter := new(test_sysinfo.MockReporter)
		reporter.On("Reject", expectedPath(t), minix, "Linux required").Return(expectedErr)

		err := p.Probe(reporter)

		reporter.AssertExpectations(t)
		assert.Same(t, expectedErr, err)
	})

	t.Run("probeErrorsOutIfUnameFails", func(t *testing.T) {
		unameResult = nil
		unameErr = expectedErr
		reporter := new(test_sysinfo.MockReporter)
		reporter.On("Error", expectedPath(t), expectedErr).Return(expectedErr)

		err := p.Probe(reporter)

		reporter.AssertExpectations(t)
		assert.Same(t, expectedErr, err)
	})

	t.Run("nestedProbes", func(t *testing.T) {
		linuxProbes.AssertKernelRelease(func(s string) string { return "" })

		t.Run("calledWhenNoError", func(t *testing.T) {
			kernelRelease := unameField{"release", false}
			unameResult = &uname{osName: linux, osRelease: kernelRelease}
			unameErr = nil

			reporter := new(test_sysinfo.MockReporter)
			reporter.On("Pass", mock.Anything, mock.Anything).Return(nil).Twice()

			err := p.Probe(reporter)

			reporter.AssertExpectations(t)
			assert.NoError(t, err)
		})

		t.Run("notCalledOnError", func(t *testing.T) {
			unameResult = nil
			unameErr = expectedErr

			reporter := new(test_sysinfo.MockReporter)
			reporter.On("Error", expectedPath(t), expectedErr).Return(expectedErr)

			err := p.Probe(reporter)

			reporter.AssertExpectations(t)
			assert.Same(t, expectedErr, err)
		})
	})
}

func TestUname(t *testing.T) {
	t.Run("smokeTest", func(t *testing.T) {
		probeUname := newUnameProber()
		uname, err := probeUname()
		if err != nil {
			t.Error(err)
		}

		t.Logf("uname: %#v", uname)
	})

	t.Run("parsesFields", func(t *testing.T) {
		var utsname syscall.Utsname
		utsname.Sysname[0] = utsChar('S')
		utsname.Nodename[0] = utsChar('N')
		utsname.Release[0] = utsChar('R')
		utsname.Version[0] = utsChar('V')
		utsname.Machine[0] = utsChar('M')

		parsed := parseUname(&utsname)
		assert.Equal(t, &uname{
			osName:    unameField{"S", false},
			nodeName:  unameField{"N", false},
			osRelease: unameField{"R", false},
			osVersion: unameField{"V", false},
			arch:      unameField{"M", false},
		}, parsed)
	})

	t.Run("parseDetectsTruncation", func(t *testing.T) {
		var utsname syscall.Utsname
		for i := range utsname.Sysname {
			utsname.Sysname[i] = utsChar('x')
		}

		parsed := parseUname(&utsname).osName
		assert.Equal(t, strings.Repeat("x", 64), parsed.value)
		assert.True(t, parsed.truncated)

		utsname.Sysname[64] = 0
		parsed = parseUname(&utsname).osName
		assert.Equal(t, strings.Repeat("x", 64), parsed.value)
		assert.True(t, parsed.truncated)
	})
}
