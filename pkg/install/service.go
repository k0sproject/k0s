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

var (
	k0sServiceName = "k0s"
	k0sDescription = "k0s - Zero Friction Kubernetes"
)

// InstalledService returns a k0s service if one has been installed on the host or an error otherwise.
func InstalledService(ctx context.Context) (sysservice.Service, error) {
	for _, role := range []string{"controller", "worker"} {
		spec, err := GetServiceSpec(role)
		if err != nil {
			return nil, err
		}
		svc, err := sysservice.New(spec)
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
	var spec sysservice.Spec
	var err error

	for _, v := range args {
		if v == "controller" || v == "worker" {
			spec, err = GetServiceSpec(v)
			if err != nil {
				return err
			}
			break
		}
	}

	svc, err := sysservice.New(spec)
	if err != nil {
		return err
	}

	spec.Args = args

	if force {
		logrus.Infof("Uninstalling %s service", spec.Name)
		err = svc.Uninstall(ctx)
		if err != nil {
			logrus.Warnf("failed to uninstall service: %v", err)
		}
	}

	logrus.Infof("Installing %s service", spec.Name)
	return svc.Install(ctx)
}

func UninstallService(role string) error {
	ctx := context.Background()
	if role == "controller+worker" {
		role = "controller"
	}

	spec, err := GetServiceSpec(role)
	if err != nil {
		return err
	}
	svc, err := sysservice.New(spec)
	if err != nil {
		return err
	}

	return svc.Uninstall(ctx)
}

func GetServiceSpec(role string) (sysservice.Spec, error) {
	var k0sDisplayName string

	if role == "controller" || role == "worker" {
		k0sDisplayName = "k0s " + role
		k0sServiceName = "k0s" + role
	}

	k0sExec, err := os.Executable()
	if err != nil {
		return sysservice.Spec{}, err
	}
	return sysservice.Spec{
		Name:        k0sServiceName,
		DisplayName: k0sDisplayName,
		Description: k0sDescription,
		Exec:        k0sExec,
	}, nil
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
			if err := svc.Restart(ctx); err != nil {
				return fmt.Errorf("failed to restart service: %w", err)
			}
			return nil
		}
		return errors.New("already running")
	}
	return svc.Start(ctx)
}
