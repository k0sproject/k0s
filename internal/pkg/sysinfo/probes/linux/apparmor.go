//go:build linux

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"slices"

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
				if err := r.Pass(desc, probes.StringProp("active")); err != nil {
					return err
				}
			default:
				return r.Warn(desc, probes.StringProp(status), "unknown")
			}

			desc = probes.NewProbeDesc("Unprivileged unconfined tasks restricted", append(slices.Clone(path), "unprivileged-unconfined-restricted"))
			restricted, supported, err := apparmor.UnprivilegedUnconfinedRestricted()
			switch {
			case err != nil:
				return r.Warn(desc, probes.ErrorProp(err), "")
			case !supported:
				return r.Pass(desc, probes.StringProp("unsupported"))
			case restricted:
				return r.Pass(desc, probes.StringProp("yes"))
			default:
				return r.Pass(desc, probes.StringProp("no"))
			}
		})
	})
}
