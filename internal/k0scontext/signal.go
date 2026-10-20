// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scontext

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// Returns a context that gets canceled as soon as k0s receives a signal to
// which it should respond with a clean shutdown.
func ShutdownContext(parent context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(parent)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs,
		syscall.SIGINT,  // Windows: CTRL_C_EVENT, CTRL_BREAK_EVENT
		syscall.SIGTERM, // Windows: CTRL_CLOSE_EVENT, CTRL_LOGOFF_EVENT, CTRL_SHUTDOWN_EVENT

		// Windows behavior:
		// https://learn.microsoft.com/en-us/windows/console/console-control-handlers
		// https://github.com/golang/go/commit/5d1a95175e693f5be0bc31ae9e6a7873318925eb#diff-fc175f04ebb256c1d34c14d27b8915f38928b71df55a35bfbd86fcb4618ff5a9
	)

	go func() {
		defer signal.Stop(sigs)
		select {
		case <-ctx.Done():
		case sig := <-sigs:
			cancel(errors.New("signal received: " + sig.String()))
		}
	}()

	return ctx, cancel
}
