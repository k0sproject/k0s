// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"errors"
	"fmt"
	"net"
)

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

type ipProp []net.IP

func (p ipProp) String() string {
	return fmt.Sprintf("%v", ([]net.IP)(p))
}
