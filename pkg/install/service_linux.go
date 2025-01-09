/*
Copyright 2025 k0s authors

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

package install

import "github.com/kardianos/service"

func configureServicePlatform(s service.Service, svcConfig *service.Config) {
	switch s.Platform() {
	case "linux-openrc":
		svcConfig.Dependencies = []string{"need cgroups", "need net", "use dns", "after firewall"}
		svcConfig.Option = map[string]interface{}{
			"OpenRCScript": openRCScript,
		}
	case "linux-upstart":
		svcConfig.Option = map[string]interface{}{
			"UpstartScript": upstartScript,
		}
	case "unix-systemv":
		svcConfig.Option = map[string]interface{}{
			"SysVScript": sysvScript,
		}
	case "linux-systemd":
		svcConfig.Dependencies = []string{"After=network-online.target", "Wants=network-online.target"}
		svcConfig.Option = map[string]interface{}{
			"SystemdScript": systemdScript,
			"LimitNOFILE":   999999,
		}
	}
}
