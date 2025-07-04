//go:build linux

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"io/ioutil"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"

	"github.com/k0sproject/k0s/internal/pkg/dir"
)

func checkAppArmor() string {
	if dir.IsDirectory("/sys/kernel/security/apparmor") {
		return "active"
	}
	lsm, err := ioutil.ReadFile("/sys/kernel/security/lsm")
	if err == nil && strings.Contains(string(lsm), "apparmor") {
		return "inactive"
	}
	return "unavailable"

}
func (l *LinuxProbes) AssertAppArmor() {
	l.Set("AppArmor", func(path probes.ProbePath, _ probes.Probe) probes.Probe {
		return probes.ProbeFn(func(r probes.Reporter) error {
			desc := probes.NewProbeDesc("AppArmor", path)
			prop := probes.StringProp(checkAppArmor())
			return r.Pass(desc, prop)
		})
	})
}
