// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelection

import (
	"context"
	"errors"
)

// Indicates that the previously gained lead has been lost.
var ErrLostLead = errors.New("lost the lead")

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
