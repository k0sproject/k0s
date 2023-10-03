//go:build linux

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
