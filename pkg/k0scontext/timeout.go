// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scontext

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/internal/sync/activity"
)

// Returns a context that times out after a specified period of inactivity.
// Calls to the keepAlive function will reset the timeout, ensuring that the
// context will remain valid for as long as there is activity.
func WithInactivityTimeout(ctx context.Context, timeout time.Duration) (_ context.Context, _ context.CancelCauseFunc, keepAlive func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	touch, last := activity.Tracker(time.Now())
	go func() {
		activity.Debounce(ctx.Done(), timeout, last, func(activity time.Time) {
			cancel(&InactivityError{activity, timeout})
		})
	}()

	return &inactivityContext{ctx}, cancel, func() { touch(time.Now()) }
}

// An error indicating that a context timed out due to inactivity.
// Will identify as [context.DeadlineExceeded] when checked by [errors.Is].
type InactivityError struct {
	LastActivity time.Time
	Timeout      time.Duration
}

func (e *InactivityError) Error() string {
	return fmt.Sprint("timed out after ", e.Timeout, " of inactivity, last activity at ", e.LastActivity)
}

func (e *InactivityError) Is(err error) bool {
	if err == context.DeadlineExceeded {
		return true
	}
	_, ok := err.(*InactivityError)
	return ok
}

// Translates causes of [*InactivityError] into [context.DeadlineExceeded].
type inactivityContext struct {
	context.Context
}

func (c *inactivityContext) Err() error {
	err := context.Cause(c.Context)
	if _, isTimeout := err.(*InactivityError); isTimeout { //nolint:errorlint
		return context.DeadlineExceeded
	}

	return c.Context.Err()
}
