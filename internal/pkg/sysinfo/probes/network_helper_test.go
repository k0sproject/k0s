// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

type netResolverFunc func(ctx context.Context, host string) ([]net.IPAddr, error)

func (f netResolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

// Creates the probe under test, looking up files below root and using a
// resolver that yields the given result. It verifies that the probe performs
// exactly one lookup.
func NewNetResolverConfigProbe(t *testing.T, ips []net.IPAddr, err error, root string, reject bool) Probe {
	var lookups uint
	t.Cleanup(func() { assert.EqualValues(t, 1, lookups) })

	return &netResolverConfigProbe{
		path: ProbePath{"networkResolverConfig"},
		resolver: netResolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
			lookups++
			assert.Equalf(t, "k0s.invalid", host, "During lookup %d", lookups)
			return ips, err
		}),
		root:   root,
		reject: reject,
	}
}
