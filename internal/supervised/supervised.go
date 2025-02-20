// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package supervised helps integrating k0s with process supervisors.
package supervised

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/k0sproject/k0s/pkg/k0scontext"
)

// Interaction points with the process supervisor.
type Interface interface {
	// Signals to the supervisor that k0s is ready. Can only be called once.
	// Subsequent calls to this method are no-ops.
	MarkReady()
}

// Gets this process's interface to its supervisor, if any.
func Get(ctx context.Context) Interface {
	return k0scontext.Value[Interface](ctx)
}

//nolint:unused // used on windows
func set(ctx context.Context, supervised Interface) context.Context {
	return k0scontext.WithValue(ctx, supervised)
}

// The main function to run in a supervised fashion.
type MainFunc func(context.Context) error

// Runs the main function in a supervisor-aware manner. The main function can
// interact with the supervisor by obtaining a supervision interface via [Get].
// Whenever the supervisor deems that k0s should exit, the context passed to
// main is canceled.
func Run(main MainFunc) error {
	return run(main)
}

// Returns a context that gets canceled as soon as k0s receives a signal to
// which it should respond with a clean shutdown.
func ShutdownContext(ctx context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(ctx)
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
