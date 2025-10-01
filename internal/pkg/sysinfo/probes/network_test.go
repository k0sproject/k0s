// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes_test

import (
	"net"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"

	test_sysinfo "github.com/k0sproject/k0s/internal/testutil/sysinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequireNameResolution(t *testing.T) {
	matchDesc := mock.MatchedBy(func(desc probes.ProbeDesc) bool {
		assert.Equal(t, "Name resolution: some-host", desc.DisplayName())
		return true
	})

	for _, test := range []struct {
		name            string
		ips             []net.IP
		err             error
		setExpectations func(*test_sysinfo.MockReporter)
		probeErr        error
	}{
		{"someIPAddress",
			[]net.IP{{127, 99, 99, 10}}, nil,
			func(r *test_sysinfo.MockReporter) {
				r.On("Pass", matchDesc, mock.MatchedBy(func(prop probes.ProbedProp) bool {
					assert.Equal(t, "[127.99.99.10]", prop.String())
					return true
				})).Return(nil)
			},
			nil,
		},
		{"noIPAddresses",
			nil, nil,
			func(r *test_sysinfo.MockReporter) {
				r.On("Error", matchDesc, mock.MatchedBy(func(err error) bool {
					if assert.Error(t, err) {
						assert.Equal(t, "no IP addresses", err.Error())
					}
					return true
				})).Return(nil)
			},
			nil,
		},
		{"lookupError",
			nil, assert.AnError,
			func(r *test_sysinfo.MockReporter) {
				r.On("Error", matchDesc, assert.AnError).Return(assert.AnError)
			},
			assert.AnError,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			reporter := new(test_sysinfo.MockReporter)
			p := probes.NewRootProbes()
			probes.RequireNameResolution(p, func(host string) ([]net.IP, error) {
				assert.Equal(t, "some-host", host)
				return test.ips, test.err
			}, "some-host")
			test.setExpectations(reporter)

			err := p.Probe(reporter)

			reporter.AssertExpectations(t)
			if test.probeErr != nil {
				assert.ErrorIs(t, err, test.probeErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
