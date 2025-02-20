// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervised

import (
	"context"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
)

func run(main MainFunc) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}

	if !isService {
		ctx, cancel := ShutdownContext(context.Background())
		defer cancel(nil)
		return main(ctx)
	}

	if err := runService(main); err != nil {
		// In case the service returns with an error,
		// log it, since stdout/stderr go into nirvana.
		logrus.WithError(err).Error("Terminating")
		return err
	}

	return nil
}
