//go:build linux

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"github.com/k0sproject/k0s/internal/os/linux/apparmor"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

func (l *LinuxProbes) AssertAppArmor() {
	l.Set("AppArmor", func(path probes.ProbePath, _ probes.Probe) probes.Probe {
		return probes.ProbeFn(func(r probes.Reporter) error {
			desc := probes.NewProbeDesc("AppArmor", path)

			status, err := apparmor.Status()
			if err != nil {
				return r.Warn(desc, probes.ErrorProp(err), "")
			}
			switch status {
			case apparmor.Unavailable:
				return r.Pass(desc, probes.StringProp("unavailable"))
			case apparmor.Disabled:
				return r.Pass(desc, probes.StringProp("disabled"))
			case apparmor.Active:
				return r.Pass(desc, probes.StringProp("active"))
			default:
				return r.Warn(desc, probes.StringProp(status), "unknown")
			}
		})
	})
}
