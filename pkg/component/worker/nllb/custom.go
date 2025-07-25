// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package nllb

import (
	"context"

	"github.com/k0sproject/k0s/internal/pkg/net"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
)

type custom struct {
	lbAddress *net.HostPort
}

func (c custom) init(_ context.Context) error                                            { return nil }
func (c custom) start(_ context.Context, _ workerconfig.Profile, _ []net.HostPort) error { return nil }
func (c custom) updateAPIServers(_ []net.HostPort) error                                 { return nil }
func (c custom) stop()                                                                   {}
func (c custom) getAPIServerAddress() (*net.HostPort, error) {
	return c.lbAddress, nil
}
