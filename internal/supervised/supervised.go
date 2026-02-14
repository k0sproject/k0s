// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package supervised helps integrating k0s with process supervisors.
package supervised

import (
	"context"

	"github.com/k0sproject/k0s/pkg/k0scontext"

	"github.com/spf13/cobra"
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

// Runs the main function in a supervisor-aware manner. The main command can
// interact with the supervisor by obtaining a supervision interface via [Get].
// Whenever the supervisor deems that k0s should exit, the context passed to
// main is canceled.
func Run(ctx context.Context, main *cobra.Command) error {
	return run(ctx, main)
}

func set(ctx context.Context, supervised Interface) context.Context {
	return k0scontext.WithValue(ctx, supervised)
}
