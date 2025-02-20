// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervised

import (
	"context"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
	"github.com/spf13/cobra"
)

func run(ctx context.Context, main *cobra.Command) error {
	isService, err := svc.IsWindowsService()

	switch {
	case err != nil:
		return err

	case isService:
		if err := runService(ctx, main); err != nil {
			// In case the service returns with an error,
			// log it, since stdout/stderr go into nirvana.
			logrus.WithError(err).Error("Terminating")
			return err
		}

		logrus.Info("Terminating")
		return nil

	default:
		return main.ExecuteContext(ctx)
	}
}
