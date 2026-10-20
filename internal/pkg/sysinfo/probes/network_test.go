// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes_test

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/net/resolvconf"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"

	test_sysinfo "github.com/k0sproject/k0s/internal/testutil/sysinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNetResolverConfig(t *testing.T) {
	matchDesc := mock.MatchedBy(func(desc probes.ProbeDesc) bool {
		assert.Equal(t, "Network resolver configuration", desc.DisplayName())
		return true
	})

	writeFile := func(t *testing.T, dir, name, content string) {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	t.Run("passes on not found errors", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		reporter.On("Pass", matchDesc, probes.StringProp("OK")).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, &net.DNSError{IsNotFound: true}, t.TempDir(), true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("warns about hijacked names", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		ips := []net.IPAddr{{IP: net.IP{198, 51, 100, 1}}}
		reporter.On("Warn", matchDesc, probes.IntoProp(ips),
			"expected NXDOMAIN, the resolver seems to hijack non-existent names",
		).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, ips, nil, t.TempDir(), true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("non DNS errors are unexpected", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		reporter.On("Warn", matchDesc, probes.ErrorProp(assert.AnError), "Unexpected error").Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, assert.AnError, t.TempDir(), true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	// This is musl's error message if no nameservers are configured and there's
	// no nameserver listening on localhost.
	tryAgain := &net.DNSError{Err: "Try again", IsTemporary: true}

	if runtime.GOOS != "linux" {
		t.Run("reports generic DNS errors on non-Linux", func(t *testing.T) {
			var reporter test_sysinfo.MockReporter
			reporter.On("Reject", matchDesc, probes.StringProp(tryAgain.Err), "DNS error").Return(nil)

			underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, t.TempDir(), true)
			assert.NoError(t, underTest.Probe(&reporter))

			reporter.AssertExpectations(t)
		})

		return
	}

	// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	// ┃                          Linux-only tests are here.                          ┃
	// ┃ The remaining cases inspect resolv.conf, which the probe only does on Linux. ┃
	// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

	t.Run("warns when configured nameservers fail", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "nameserver 1.2.3.4\nnameserver fe80::1\n")
		reporter.On("Warn", matchDesc, probes.StringProp(tryAgain.Err),
			"check the configured nameservers",
		).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("warns when systemd-resolved stub doesn't answer", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "nameserver 127.0.0.53\n")
		reporter.On("Warn", matchDesc, probes.StringProp(tryAgain.Err),
			"/etc/resolv.conf points to systemd-resolved's stub resolver, check that systemd-resolved is running",
		).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("warns when systemd-resolved stub doesn't answer despite uplink file", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "nameserver 127.0.0.53\n")
		writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "nameserver 1.2.3.4\n")
		reporter.On("Warn", matchDesc, probes.StringProp(tryAgain.Err),
			"/etc/resolv.conf points to systemd-resolved's stub resolver, check that systemd-resolved is running",
		).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("rejects without nameservers", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "# Resolver configuration file.\n# See resolv.conf(5) for details.\n")
		reporter.On("Reject", matchDesc, probes.StringProp(tryAgain.Err), "no nameservers configured, DNS queries are sent to localhost").Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("warns without nameservers if not rejecting", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "")
		reporter.On("Warn", matchDesc, probes.StringProp(tryAgain.Err), "no nameservers configured, DNS queries are sent to localhost").Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, false)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("treats nonexistent resolv.conf as empty", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		reporter.On("Reject", matchDesc, probes.StringProp(tryAgain.Err), "/etc/resolv.conf doesn't exist, DNS queries are sent to localhost").Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("hints at systemd-resolved stub", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "")
		writeFile(t, root, resolvconf.SystemdResolvedStubPath, "nameserver 127.0.0.53\n")
		writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "nameserver 1.2.3.4\n")
		reporter.On("Reject", matchDesc, probes.StringProp(tryAgain.Err), "no nameservers configured, DNS queries are sent to localhost"+
			"; systemd-resolved seems to be running, but /etc/resolv.conf isn't managed by it"+
			", consider symlinking it to /run/systemd/resolve/stub-resolv.conf",
		).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("doesn't hint at nonexistent systemd-resolved stub", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "")
		writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "nameserver 1.2.3.4\n")
		reporter.On("Reject", matchDesc, probes.StringProp(tryAgain.Err), "no nameservers configured, DNS queries are sent to localhost").Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("doesn't hint at systemd-resolved stub if resolv.conf is the uplink file", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		writeFile(t, root, resolvconf.SystemdResolvedStubPath, "nameserver 127.0.0.53\n")
		writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "")
		resolvConfPath := filepath.Join(root, resolvconf.Path)
		require.NoError(t, os.MkdirAll(filepath.Dir(resolvConfPath), 0700))
		require.NoError(t, os.Symlink(filepath.Join(root, resolvconf.SystemdResolvedUplinkPath), resolvConfPath))
		reporter.On("Reject", matchDesc, probes.StringProp(tryAgain.Err), "systemd-resolved has no nameservers configured, DNS queries are sent to localhost").Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("warns if resolv.conf cannot be opened", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := filepath.Join(t.TempDir(), strings.Repeat("x", 256))
		reporter.On("Warn", matchDesc, probes.StringProp(tryAgain.Err), mock.MatchedBy(func(msg string) bool {
			assert.Contains(t, msg, "DNS error (open ")
			assert.Contains(t, msg, "/etc/resolv.conf: file name too long")
			return true
		})).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})

	t.Run("warns if resolv.conf cannot be read", func(t *testing.T) {
		var reporter test_sysinfo.MockReporter
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, resolvconf.Path), 0700)) // reading a directory fails
		reporter.On("Warn", matchDesc, probes.StringProp(tryAgain.Err), mock.MatchedBy(func(msg string) bool {
			assert.Contains(t, msg, "failed to detect systemd-resolved: read")
			assert.Contains(t, msg, "is a directory")
			return true
		})).Return(nil)

		underTest := probes.NewNetResolverConfigProbe(t, nil, tryAgain, root, true)
		assert.NoError(t, underTest.Probe(&reporter))

		reporter.AssertExpectations(t)
	})
}

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
