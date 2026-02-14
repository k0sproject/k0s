// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import "github.com/kardianos/service"

func configureServicePlatform(s service.Service, svcConfig *service.Config) {
	switch s.Platform() {
	case "linux-openrc":
		svcConfig.Dependencies = []string{"need cgroups", "need net", "use dns", "after firewall"}
		svcConfig.Option = map[string]any{
			"OpenRCScript": openRCScript,
		}
	case "linux-upstart":
		svcConfig.Option = map[string]any{
			"UpstartScript": upstartScript,
		}
	case "unix-systemv":
		svcConfig.Option = map[string]any{
			"SysVScript": sysvScript,
		}
	case "linux-systemd":
		svcConfig.Dependencies = []string{"After=network-online.target", "Wants=network-online.target"}
		svcConfig.Option = map[string]any{
			"SystemdScript": systemdScript,
			"LimitNOFILE":   999999,
		}
	}
}
