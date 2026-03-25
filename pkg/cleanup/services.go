//go:build linux || windows

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"io/fs"
	"os/exec"

	"github.com/k0sproject/k0s/pkg/install"
	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

type services struct{}

// Name returns the name of the step
func (s *services) Name() string {
	return "uninstall service step"
}

// Run uninstalls k0s services that are found on the host
func (s *services) Run() error {
	var errs []error

	for _, role := range []string{"controller", "worker"} {
		logrus.Debugf("attempting to uninstall k0s%s service", role)
		if err := install.UninstallService(role); err != nil {
			if !errors.Is(err, service.ErrNotInstalled) && !errors.Is(err, fs.ErrNotExist) && !isExitCode(err, 1) {
				errs = append(errs, err)
			}
		} else {
			logrus.Infof("uninstalled k0s%s service", role)
			return nil
		}
	}

	return errors.Join(errs...)
}

func isExitCode(err error, exitcode int) bool {
	var e *exec.ExitError
	return errors.As(err, &e) && e.ExitCode() == exitcode
}
