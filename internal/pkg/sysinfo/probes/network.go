/*
Copyright 2023 k0s authors

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

package probes

import (
	"errors"
	"fmt"
	"net"
)

func RequireNameResolution(p Probes, lookupIP func(host string) ([]net.IP, error), host string) {
	p.Set(fmt.Sprintf("nameResolution:%s", host), func(path ProbePath, _ Probe) Probe {
		return ProbeFn(func(r Reporter) error {
			desc := NewProbeDesc(fmt.Sprintf("Name resolution: %s", host), path)
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
