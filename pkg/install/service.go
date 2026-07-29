// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
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
func InstallService(ctx context.Context, args []string, envVars []string, force bool) error {
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
		if runtime.GOOS == "windows" {
			// On Windows, the service must be stopped first
			if err = s.Stop(); err != nil && !errors.Is(err, service.ErrNotInstalled) {
				logrus.Warnf("failed to stop service before re-install: %v", err)
			}
		}

		logrus.Infof("Uninstalling %s service", svcConfig.Name)
		err = s.Uninstall()
		if err != nil && !errors.Is(err, service.ErrNotInstalled) {
			logrus.Warnf("failed to uninstall service: %v", err)
		}
		// On windows the service delete/uninstall is async, wait till it is done
		if runtime.GOOS == "windows" {
			if err = wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
				_, err = s.Status()
				if errors.Is(err, service.ErrNotInstalled) {
					return true, nil
				}
				logrus.Info("waiting for the service to be actually deleted")
				return false, nil
			}); err != nil {
				logrus.Errorf("failed to wait service to be deleted: %v", err)
				return err
			}
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

// StartInstalledService starts (or restarts with force) the installed k0s service.
func StartInstalledService(force bool) error {
	svc, err := InstalledService()
	if err != nil {
		return err
	}
	status, _ := svc.Status()
	if status == service.StatusRunning {
		if force {
			if err := svc.Restart(); err != nil {
				return fmt.Errorf("failed to restart service: %w", err)
			}
			return nil
		}
		return errors.New("already running")
	}
	return svc.Start()
}
