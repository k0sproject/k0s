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

package k0scontext_test

import (
	"context"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithInactivityTimeout_KeepAlive(t *testing.T) {
	const (
		jitter  = 25 * time.Millisecond // tolerable error for assertions
		timeout = 10 * jitter           // inactivity timeout for the context
		delay   = timeout / 2           // delay after which the timeout is kept alive once
	)

	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)

	// Create a new context. The timeout is ticking ...
	ctx, _, keepAlive := k0scontext.WithInactivityTimeout(ctx, timeout)

	// Wait for some time, then keep the context alive once.
	time.Sleep(delay)
	start := time.Now()
	keepAlive()
	keptAlive := time.Now()

	// Make sure the timeout has not expired.
	select {
	case <-ctx.Done():
		require.Fail(t, "Context already done, increase the jitter")
	default:
	}

	// Now wait for the timeout to expire.
	<-ctx.Done()
	done := time.Now()

	var inactivityErr *k0scontext.InactivityError
	require.ErrorAs(t, context.Cause(ctx), &inactivityErr)
	assert.Equal(t, timeout, inactivityErr.Timeout)
	assert.WithinRange(t, inactivityErr.LastActivity, start, keptAlive)
	assert.Less(t, keptAlive.Sub(start), jitter, "Touching took too long, increase the jitter")
	assert.WithinRange(t, done,
		// The earliest tolerable done time: The time just before the context
		// was last kept alive, plus the timeout itself.
		start.Add(timeout),
		// The latest tolerable done time: The time just after the context was
		// kept alive, plus the timeout itself, plus the jitter.
		keptAlive.Add(timeout).Add(jitter),
	)
}

func TestWithInactivityTimeout_Timeout(t *testing.T) {
	ctx, cancel, _ := k0scontext.WithInactivityTimeout(context.TODO(), 0)
	t.Cleanup(func() { cancel(nil) })

	<-ctx.Done()
	err, cause := ctx.Err(), context.Cause(ctx)

	assert.Equal(t, context.DeadlineExceeded, err)
	assert.ErrorIs(t, cause, context.DeadlineExceeded)
	assert.ErrorContains(t, cause, "timed out after 0s of inactivity, last activity at ")
}

func TestWithInactivityTimeout_Cancel(t *testing.T) {
	t.Run("Self", func(t *testing.T) {
		ctx, cancel, _ := k0scontext.WithInactivityTimeout(context.TODO(), time.Hour)
		t.Cleanup(func() { cancel(nil) })

		cancel(assert.AnError)
		<-ctx.Done()

		assert.Equal(t, context.Canceled, ctx.Err())
		assert.Equal(t, assert.AnError, context.Cause(ctx))
	})

	t.Run("Outer", func(t *testing.T) {
		ctx, outerCancel := context.WithCancelCause(context.TODO())
		t.Cleanup(func() { outerCancel(nil) })

		ctx, cancel, _ := k0scontext.WithInactivityTimeout(ctx, time.Hour)
		t.Cleanup(func() { cancel(nil) })

		outerCancel(assert.AnError)
		<-ctx.Done()

		assert.Equal(t, context.Canceled, ctx.Err())
		assert.Equal(t, assert.AnError, context.Cause(ctx))
	})
}
