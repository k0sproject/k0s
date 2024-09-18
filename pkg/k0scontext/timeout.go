/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k0scontext

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Returns a context that times out after a specified period of inactivity.
// Calls to the keepAlive function will reset the timeout, ensuring that the
// context will remain valid for as long as there is activity.
func WithInactivityTimeout(ctx context.Context, timeout time.Duration) (_ context.Context, _ context.CancelCauseFunc, keepAlive func()) {
	var lastActivity atomic.Pointer[time.Time]
	keepAlive = func() {
		now := time.Now()
		lastActivity.Store(&now)
	}

	ctx, cancel := context.WithCancelCause(ctx)
	keepAlive() // initialize the pointer

	go func() {
		for {
			lastActivity := *lastActivity.Load()
			remaining := time.Until(lastActivity.Add(timeout))

			if remaining <= 0 {
				cancel(&InactivityError{lastActivity, timeout})
				return
			}

			select {
			// Recalculate timeout to minimize drift.
			case <-time.After(time.Until(lastActivity.Add(timeout))):
			case <-ctx.Done():
				return
			}
		}
	}()

	return &inactivityContext{ctx}, cancel, keepAlive
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
