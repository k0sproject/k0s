// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/k0sproject/k0s/pkg/sysservice"
	"github.com/sirupsen/logrus"
)

const (
	roleController = "controller"
	roleWorker     = "worker"
)

// InstalledService returns a k0s service if one has been installed on the host or an error otherwise.
func InstalledService(ctx context.Context) (sysservice.Service, error) {
	for _, role := range []string{roleController, roleWorker} {
		svc, err := sysservice.New("k0s" + role)
		if err != nil {
			return nil, err
		}

		st, err := svc.Status(ctx)
		if err != nil {
			return nil, err
		}
		if st != sysservice.StatusNotInstalled {
			return svc, nil
		}
	}

	return nil, errors.New("k0s has not been installed as a service")
}

// InstallService installs the k0s service, per the given arguments, and the detected platform
func InstallService(ctx context.Context, args []string, envVars []string, force bool) error {
	var name string
	for _, v := range args {
		if v == roleController || v == roleWorker {
			name = "k0s" + v
			break
		}
	}
	if name == "" {
		return errors.New("args must contain a role (controller or worker)")
	}

	svc, err := sysservice.New(name)
	if err != nil {
		return err
	}

	if force {
		logrus.Infof("Uninstalling %s service", name)
		if err := svc.Uninstall(ctx); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Warnf("failed to uninstall service: %v", err)
		}
	}

	logrus.Infof("Installing %s service", name)
	err = svc.Install(ctx, args, envVars)
	if err != nil {
		return err
	}
	logrus.Infof("Enabling %s service", name)
	return svc.Enable(ctx)
}

func UninstallService(ctx context.Context, role string) error {
	if role == "controller+worker" {
		role = roleController
	}

	svc, err := sysservice.New("k0s" + role)
	if err != nil {
		return err
	}

	return svc.Uninstall(ctx)
}

// StartInstalledService starts (or restarts with force) the installed k0s service.
func StartInstalledService(ctx context.Context, force bool) error {
	svc, err := InstalledService(ctx)
	if err != nil {
		return err
	}
	status, _ := svc.Status(ctx)
	if status == sysservice.StatusRunning {
		if force {
			if err := svc.Stop(ctx); err != nil {
				return fmt.Errorf("failed to stop service: %w", err)
			}
		} else {
			return errors.New("already running")
		}
	}
	return svc.Start(ctx)
}
