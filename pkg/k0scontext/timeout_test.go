// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scontext_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithInactivityTimeout_KeepAlive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a new context. The timeout is ticking ...
		ctx, _, keepAlive := k0scontext.WithInactivityTimeout(t.Context(), time.Second)

		// Send a keep-alive just before the timeout exceeds
		time.Sleep(time.Second - time.Nanosecond)
		lastActivity := time.Now()
		keepAlive()

		// Make sure the timeout has not expired.
		synctest.Wait()
		assert.NoError(t, context.Cause(ctx), "Context done, despite a keep-alive")

		// Push the timeout over the brink now.
		time.Sleep(time.Second)
		synctest.Wait()
		var inactivityErr *k0scontext.InactivityError
		require.ErrorAs(t, context.Cause(ctx), &inactivityErr)
		assert.Equal(t, time.Second, inactivityErr.Timeout)
		assert.Equal(t, lastActivity, inactivityErr.LastActivity)
	})
}

func TestWithInactivityTimeout_Timeout(t *testing.T) {
	ctx, _, _ := k0scontext.WithInactivityTimeout(t.Context(), 0)

	<-ctx.Done()
	err, cause := ctx.Err(), context.Cause(ctx)

	assert.Equal(t, context.DeadlineExceeded, err)
	assert.ErrorIs(t, cause, context.DeadlineExceeded)
	assert.ErrorContains(t, cause, "timed out after 0s of inactivity, last activity at ")
}

func TestWithInactivityTimeout_Cancel(t *testing.T) {
	t.Run("Self", func(t *testing.T) {
		ctx, cancel, _ := k0scontext.WithInactivityTimeout(t.Context(), time.Hour)

		cancel(assert.AnError)
		<-ctx.Done()

		assert.Equal(t, context.Canceled, ctx.Err())
		assert.Equal(t, assert.AnError, context.Cause(ctx))
	})

	t.Run("Outer", func(t *testing.T) {
		ctx, outerCancel := context.WithCancelCause(t.Context())
		ctx, _, _ = k0scontext.WithInactivityTimeout(ctx, time.Hour)

		outerCancel(assert.AnError)
		<-ctx.Done()

		assert.Equal(t, context.Canceled, ctx.Err())
		assert.Equal(t, assert.AnError, context.Cause(ctx))
	})
}
