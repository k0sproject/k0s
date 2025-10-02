// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"errors"
	"runtime"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

var (
	k0sServiceName = "k0s"
	k0sDescription = "k0s - Zero Friction Kubernetes"
)

type Program struct{}

func (p *Program) Start(service.Service) error {
	// Start should not block. Do the actual work async.
	return nil
}

func (p *Program) Stop(service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

// InstalledService returns a k0s service if one has been installed on the host or an error otherwise.
func InstalledService() (service.Service, error) {
	prg := &Program{}
	for _, role := range []string{"controller", "worker"} {
		c := GetServiceConfig(role)
		s, err := service.New(prg, c)
		if err != nil {
			return nil, err
		}
		_, err = s.Status()

		if err != nil && errors.Is(err, service.ErrNotInstalled) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return s, nil
	}

	var s service.Service
	return s, errors.New("k0s has not been installed as a service")
}

// InstallService installs the k0s service, per the given arguments, and the detected platform
func InstallService(args []string, envVars []string, force bool) error {
	var svcConfig *service.Config

	prg := &Program{}
	for _, v := range args {
		if v == "controller" || v == "worker" {
			svcConfig = GetServiceConfig(v)
			break
		}
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	configureServicePlatform(s, svcConfig)

	if len(envVars) > 0 {
		svcConfig.Option["Environment"] = envVars
	}

	if runtime.GOOS == "windows" {
		args = append([]string{"service=" + svcConfig.Name}, args...)
	}

	svcConfig.Arguments = args

	if force {
		logrus.Infof("Uninstalling %s service", svcConfig.Name)
		err = s.Uninstall()
		if err != nil && !errors.Is(err, service.ErrNotInstalled) {
			logrus.Warnf("failed to uninstall service: %v", err)
		}
	}

	logrus.Infof("Installing %s service", svcConfig.Name)
	return s.Install()
}

func UninstallService(role string) error {
	prg := &Program{}

	if role == "controller+worker" {
		role = "controller"
	}

	svcConfig := GetServiceConfig(role)
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	return s.Uninstall()
}

func GetServiceConfig(role string) *service.Config {
	var k0sDisplayName string

	if role == "controller" || role == "worker" {
		k0sDisplayName = "k0s " + role
		k0sServiceName = "k0s" + role
	}
	return &service.Config{
		Name:        k0sServiceName,
		DisplayName: k0sDisplayName,
		Description: k0sDescription,
	}
}
