//go:build linux || windows

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"errors"

	"github.com/k0sproject/k0s/pkg/install"
	"github.com/k0sproject/k0s/pkg/sysservice"
	"github.com/sirupsen/logrus"
)

type services struct{}

// Name returns the name of the step
func (s *services) Name() string {
	return "uninstall service step"
}

// Run uninstalls k0s services that are found on the host
func (s *services) Run(ctx context.Context) error {
	var errs []error

	for _, role := range []string{"controller", "worker"} {
		logrus.Debugf("attempting to uninstall k0s%s service", role)
		if err := install.UninstallService(ctx, role); err != nil {
			// Nothing to clean up if there's no service, and none can have
			// been installed in the first place without a supported init
			// system.
			if !errors.Is(err, sysservice.ErrNotInstalled) && !errors.Is(err, sysservice.ErrUnsupportedInitSystem) {
				errs = append(errs, err)
			}
		} else {
			logrus.Infof("uninstalled k0s%s service", role)
			return nil
		}
	}

	return errors.Join(errs...)
}
