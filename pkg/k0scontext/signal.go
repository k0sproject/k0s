// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scontext

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func ShutdownContext(parent context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(parent)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(sigs)
		select {
		case <-ctx.Done():
		case sig := <-sigs:
			cancel(fmt.Errorf("signal received: %s", sig))
		}
	}()

	return ctx, cancel
}
