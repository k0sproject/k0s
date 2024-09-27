/*
Copyright 2020 k0s authors

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

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/service"
	"github.com/sirupsen/logrus"
)

// EnsureService installs the k0s service, per the given arguments, and the detected platform
func EnsureService(args []string, envVars []string, force bool) error {
	var deps []string
	var svcConfig *service.Config

	for _, v := range args {
		if v == "controller" || v == "worker" {
			svcConfig = service.K0sConfig(v)
			break
		}
	}

	s, err := service.NewService(svcConfig)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// fetch service type
	svcType := s.Platform()
	switch svcType {
	case "linux-openrc":
		deps = []string{"need cgroups", "need net", "use dns", "after firewall"}
		svcConfig.Option = map[string]interface{}{
			"OpenRCScript": openRCScript,
		}
	case "linux-upstart":
		svcConfig.Option = map[string]interface{}{
			"UpstartScript": upstartScript,
		}
	case "unix-systemv":
		svcConfig.Option = map[string]interface{}{
			"SystemdScript": sysvScript,
		}
	case "linux-systemd":
		deps = []string{"After=network-online.target", "Wants=network-online.target"}
		svcConfig.Option = map[string]interface{}{
			"SystemdScript": systemdScript,
			"LimitNOFILE":   999999,
		}
	default:
	}

	if len(envVars) > 0 {
		svcConfig.Option["Environment"] = envVars
	}

	svcConfig.Dependencies = deps
	svcConfig.Arguments = args
	if force {
		logrus.Infof("Uninstalling %s service", svcConfig.Name)
		status, err := s.Status()
		if err != nil {
			logrus.Warnf("failed to get service status: %v", err)
		}
		if status != service.StatusNotInstalled {
			if err := s.Uninstall(); err != nil {
				logrus.Warnf("failed to uninstall service: %v", err)
			}
		}
	}
	logrus.Infof("Installing %s service", svcConfig.Name)
	err = s.Install()
	if err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}
	return nil
}

func UninstallService(role string) error {
	s, err := service.InstalledK0sService()
	if err != nil {
		return fmt.Errorf("uninstall service: %w", err)
	}

	if err := s.Uninstall(); err != nil {
		return fmt.Errorf("uninstall service: %w", err)
	}

	return nil
}

func prepareEnvVars(envVars []string) map[string]string {
	result := make(map[string]string)
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 1)
		if len(parts) != 2 {
			continue
		}

		result[parts[0]] = parts[1]
	}
	return result
}
