// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelection

import (
	"context"
	"errors"
)

// Indicates that the previously gained lead has been lost.
var ErrLostLead = errors.New("lost the lead")

// Indicates that the lead wasn't held to begin with.
var ErrNotLeading = errors.New("not currently leading")

// Returns the current leader election status. Whenever the status becomes
// outdated, the returned expired channel will be closed.
type StatusFunc func() (current Status, expired <-chan struct{})

// Runs the provided tasks function when the lead is taken. It continuously
// monitors the leader election status using statusFunc. When the lead is taken,
// the tasks function is called with a context that is canceled either when the
// lead has been lost or ctx is done. After the tasks function returns, the
// process is repeated until ctx is done.
func RunLeaderTasks(ctx context.Context, statusFunc StatusFunc, tasks func(context.Context)) {
	for {
		status, statusExpired := statusFunc()

		if status == StatusLeading {
			ctx, cancel := context.WithCancelCause(ctx)
			go func() {
				select {
				case <-statusExpired:
					cancel(ErrLostLead)
				case <-ctx.Done():
				}
			}()

			tasks(ctx)
		}

		select {
		case <-statusExpired:
		case <-ctx.Done():
			return
		}
	}
}

// Derives a context from ctx for a single leadership snapshot, taken via
// statusFunc. The returned context is already canceled with ErrNotLeading if
// the caller isn't currently leading. Otherwise, it gets canceled with
// ErrLostLead as soon as the lease ends, or whenever ctx itself is done.
// Unlike RunLeaderTasks, this doesn't loop or wait for the lead to be taken;
// callers are expected to call it again for every new leadership check. The
// returned cancel func must be called once the context is no longer needed,
// to release resources.
func LeaderContext(ctx context.Context, statusFunc StatusFunc) (context.Context, context.CancelFunc) {
	status, expired := statusFunc()

	leaderCtx, cancel := context.WithCancelCause(ctx)
	stop := func() { cancel(nil) }
	if status != StatusLeading {
		cancel(ErrNotLeading)
		return leaderCtx, stop
	}

	go func() {
		select {
		case <-expired:
			cancel(ErrLostLead)
		case <-leaderCtx.Done():
		}
	}()

	return leaderCtx, stop
}
