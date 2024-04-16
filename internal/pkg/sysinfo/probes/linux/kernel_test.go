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
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"

	test_sysinfo "github.com/k0sproject/k0s/internal/testutil/sysinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequireKernelConfig(t *testing.T) {

	linux := newLinuxProbes(probes.ProbePath{})

	type configResult struct {
		o kConfigOption
		e error
	}

	var c map[string]configResult
	linux.probeKConfig = func(config kConfig) (kConfigOption, error) {
		r := c[string(config)]
		return r.o, r.e
	}

	t.Run("invalidKConfigPanics", func(t *testing.T) {
		assert.PanicsWithValue(t, `invalid kernel config: "foo"`, func() {
			linux.RequireKernelConfig("foo", "")
		})
	})

	t.Run("nestedConfigs", func(t *testing.T) {
		linux.RequireKernelConfig("IKCONFIG", "ikconfig").RequireKernelConfig("IKCONFIG_PROC", "ikconfig_proc")

		c = map[string]configResult{
			"IKCONFIG":      {kConfigBuiltIn, nil},
			"IKCONFIG_PROC": {kConfigAsModule, nil},
		}

		t.Run("calledWhenNoError", func(t *testing.T) {
			reporter := new(test_sysinfo.MockReporter)
			reporter.On("Pass", mock.MatchedBy(func(desc probes.ProbeDesc) bool {
				if (probes.ProbePath{"IKCONFIG"}).Equal(desc.Path()) {
					assert.Equal(t, "CONFIG_IKCONFIG: ikconfig", desc.DisplayName())
					return true
				}
				return false
			}), kConfigBuiltIn).Return(nil)
			reporter.On("Pass", mock.MatchedBy(func(desc probes.ProbeDesc) bool {
				if (probes.ProbePath{"IKCONFIG", "IKCONFIG_PROC"}).Equal(desc.Path()) {
					assert.Equal(t, "CONFIG_IKCONFIG_PROC: ikconfig_proc", desc.DisplayName())
					return true
				}
				return false
			}), kConfigAsModule).Return(nil)

			err := linux.Probes.Probe(reporter)

			reporter.AssertExpectations(t)
			assert.NoError(t, err)
		})

		t.Run("notCalledOnError", func(t *testing.T) {
			expectedErr := errors.New("dummy")
			reporter := new(test_sysinfo.MockReporter)
			reporter.On("Pass", mock.MatchedBy(func(desc probes.ProbeDesc) bool {
				return probes.ProbePath{"IKCONFIG"}.Equal(desc.Path())
			}), kConfigBuiltIn).Return(expectedErr)

			err := linux.Probes.Probe(reporter)

			reporter.AssertExpectations(t)
			assert.Same(t, expectedErr, err)
		})

		t.Run("warnsIfNotFound", func(t *testing.T) {
			var expectedErr noKConfigsFound
			c["IKCONFIG"] = configResult{kConfigUnknown, &expectedErr}
			reporter := new(test_sysinfo.MockReporter)
			reporter.On("Warn", mock.Anything, &expectedErr, "").Return(nil)

			err := linux.Probes.Probe(reporter)

			reporter.AssertExpectations(t)
			assert.NoError(t, err)
		})
	})
}

func TestKConfigProber(t *testing.T) {

	// This may fail on systems that don't expose their kernel config at runtime.
	t.Run("smokeTest", func(t *testing.T) {
		probeKConfig := newKConfigProber(newUnameProber())

		option, err := probeKConfig(ensureKConfig("I_CERTAINLY_DONT_EXIST"))
		assert.Equal(t, option, kConfigUnknown)
		var notFoundErr *noKConfigsFound
		if errors.As(err, &notFoundErr) {
			t.Logf("System doesn't seem to expose its kernel config: %v", err)
		} else {
			assert.NoError(t, err)
		}
	})

	t.Run("propagatesUnameErr", func(t *testing.T) {
		unameErr := errors.New("what uname?")
		probeKConfig := newKConfigProber(func() (*uname, error) {
			return nil, unameErr
		})

		option, err := probeKConfig(ensureKConfig("FOO"))
		assert.Equal(t, kConfigUnknown, option)
		assert.Same(t, unameErr, err)
	})
}

func TestKernelConfigParser(t *testing.T) {
	//revive:disable-next-line:var-naming
	kConfigData := `#
# Automatically generated file; DO NOT EDIT.
# Linux/x86_64 5.16.8 Kernel Configuration
#
CONFIG_CC_VERSION_TEXT="gcc (GCC) 10.3.0"
CONFIG_CC_IS_GCC=y
CONFIG_GCC_VERSION=100300
CONFIG_CLANG_VERSION=0
CONFIG_AS_IS_GNU=y
CONFIG_AS_VERSION=23502

CONFIG_IKCONFIG=y
CONFIG_IKCONFIG_PROC=m
CONFIG_IKHEADERS=n
`

	r := strings.NewReader(kConfigData)
	assert := assert.New(t)
	x, err := parseKConfigs(r)
	assert.NoError(err)
	assert.Equal(kConfigs{
		ensureKConfig("CC_IS_GCC"):     kConfigBuiltIn,
		ensureKConfig("AS_IS_GNU"):     kConfigBuiltIn,
		ensureKConfig("IKCONFIG"):      kConfigBuiltIn,
		ensureKConfig("IKCONFIG_PROC"): kConfigAsModule,
		ensureKConfig("IKHEADERS"):     kConfigLeftOut,
	}, x)
}
