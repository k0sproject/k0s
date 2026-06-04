// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"time"

	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

func watchDir(ctx context.Context, w *dirWatch) error {
	err, fallback := w.runFSNotify(ctx)
	if err == nil || !fallback {
		return err
	}

	log := k0scontext.ValueOrElse(ctx, func() logrus.FieldLogger {
		return logrus.StandardLogger().WithFields(logrus.Fields{
			"component": "os/watch",
			"path":      w.path,
		})
	})

	const pollInterval = 30 * time.Second
	log.WithError(err).Warn("Falling back to polling every ", pollInterval)
	return w.runPolling(log, ctx.Done(), func() time.Duration {
		return wait.Jitter(pollInterval, 0.2)
	})
}
