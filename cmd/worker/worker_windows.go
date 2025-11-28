// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/k0scontext"
)

func initLogging(ctx context.Context, logDir string) error {
	backend := k0scontext.Value[log.Backend](ctx)
	if backend == nil {
		return nil
	}
	logFileBackend, ok := backend.(log.LogFileBackend)
	if !ok {
		return nil
	}

	if err := logFileBackend.InitLogFile(func() (*os.File, error) {
		// Just create a dumb log file in the given parent directory for now.
		// No rotation, no cleanup.

		if err := dir.Init(logDir, constant.DataDirMode); err != nil {
			return nil, err
		}

		return os.CreateTemp(logDir, fmt.Sprintf("k0s_%d_*.log", time.Now().Unix()))
	}); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}

	return nil
}

func addPlatformSpecificComponents(ctx context.Context, m *manager.Manager, k0sVars *config.CfgVars, controller EmbeddingController, certManager *worker.CertificateManager) {
	if controller != nil {
		return
	}

	m.Add(ctx, &status.Status{
		Prober: prober.DefaultProber,
		StatusInformation: status.K0sStatus{
			Pid:        os.Getpid(),
			Role:       "worker",
			Args:       os.Args,
			Version:    build.Version,
			Workloads:  true,
			SingleNode: false,
			K0sVars:    k0sVars,
		},
		CertManager: certManager,
		Socket:      k0sVars.StatusSocketPath,
	})
}
