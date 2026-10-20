// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/net/resolvconf"
)

type netResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// Requires that a DNS resolver answers queries for this host. It rejects the
// host if the resolver doesn't answer and the given resolv.conf file doesn't
// list any nameservers, which is a persistent misconfiguration rather than a
// connectivity problem. Everything else that doesn't look healthy is a warning.
func RequireNetResolverConfig(parent ParentProbe) {
	addNetResolverConfigProbe(parent, true)
}

// Like [RequireNetResolverConfig], but only ever warns.
func AssertNetResolverConfig(parent ParentProbe) {
	addNetResolverConfigProbe(parent, false)
}

func addNetResolverConfigProbe(parent ParentProbe, reject bool) {
	parent.Set("networkResolverConfig", func(path ProbePath, _ Probe) Probe {
		return &netResolverConfigProbe{path, net.DefaultResolver, "/", reject}
	})
}

func RequireNameResolution(p Probes, lookupIP func(host string) ([]net.IP, error), host string) {
	p.Set("nameResolution:"+host, func(path ProbePath, _ Probe) Probe {
		return ProbeFn(func(r Reporter) error {
			desc := NewProbeDesc("Name resolution: "+host, path)
			ips, err := lookupIP(host)
			if err != nil {
				return r.Error(desc, err)
			}
			if len(ips) < 1 {
				return r.Error(desc, errors.New("no IP addresses"))
			}

			return r.Pass(desc, ipProp(ips))
		})
	})
}

type netResolverConfigProbe struct {
	path     ProbePath
	resolver netResolver
	root     string // The well-known files are looked up below this directory.
	reject   bool
}

func (p *netResolverConfigProbe) Probe(r Reporter) error {
	desc := NewProbeDesc("Network resolver configuration", p.path)

	// Resolvers facing a broken config may need a while to give up;
	// musl takes up to 10 s in its default configuration.
	ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
	defer cancel()

	// The "invalid" top-level domain is reserved by RFC 6761. Resolvers are
	// expected to answer NXDOMAIN for names under it without asking the
	// network, so any working resolver configuration will fail this lookup
	// quickly, and it doesn't depend on connectivity.
	// https://www.rfc-editor.org/rfc/rfc6761#section-6.4
	ips, err := p.resolver.LookupIPAddr(ctx, "k0s.invalid")
	if err == nil {
		return r.Warn(desc, IntoProp(ips), "expected NXDOMAIN, the resolver seems to hijack non-existent names")
	}

	var prop ProbedProp
	if dnsErr, ok := errors.AsType[*net.DNSError](err); !ok {
		return r.Warn(desc, ErrorProp(err), "Unexpected error")
	} else {
		if dnsErr.IsNotFound {
			return r.Pass(desc, StringProp("OK"))
		}
		prop = StringProp(dnsErr.Err)
	}

	var msg string

	if runtime.GOOS == "linux" {
		if resolvConf, err := os.Open(filepath.Join(p.root, resolvconf.Path)); err == nil {
			defer resolvConf.Close()
			if isStub, err := resolvconf.IsSystemdResolvedStub(resolvConf); errors.Is(err, resolvconf.ErrNoNameservers) {
				msg = resolvconf.ErrNoNameservers.Error() + ", DNS queries are sent to localhost"
			} else if err != nil {
				return r.Warn(desc, prop, "failed to detect systemd-resolved: "+err.Error())
			} else if isStub {
				return r.Warn(desc, prop, resolvconf.Path+" points to systemd-resolved's stub resolver, check that systemd-resolved is running")
			} else {
				return r.Warn(desc, prop, "check the configured nameservers")
			}

			// Recommend the stub file, as systemd does. k0s configures the
			// kubelet to use the uplink file for pods if it detects the stub.
			if uplinkInfo, err := os.Stat(filepath.Join(p.root, resolvconf.SystemdResolvedUplinkPath)); err == nil {
				if info, err := resolvConf.Stat(); err == nil && os.SameFile(info, uplinkInfo) {
					msg = "systemd-resolved has " + msg
				} else if _, err := os.Stat(filepath.Join(p.root, resolvconf.SystemdResolvedStubPath)); err == nil {
					msg += "; systemd-resolved seems to be running"
					msg += ", but " + resolvconf.Path + " isn't managed by it"
					msg += ", consider symlinking it to " + resolvconf.SystemdResolvedStubPath
				}
			}
		} else if errors.Is(err, os.ErrNotExist) {
			msg = resolvconf.Path + " doesn't exist, DNS queries are sent to localhost"
		} else {
			return r.Warn(desc, prop, fmt.Sprintf("DNS error (%v)", err))
		}
	} else {
		msg = "DNS error"
	}

	if p.reject {
		return r.Reject(desc, prop, msg)
	}
	return r.Warn(desc, prop, msg)
}

type ipProp []net.IP

func (p ipProp) String() string {
	return fmt.Sprintf("%v", ([]net.IP)(p))
}
