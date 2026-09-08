// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLeaderContext_NotLeading(t *testing.T) {
	statusFunc := func() (Status, <-chan struct{}) { return StatusPending, nil }

	ctx, cancel := LeaderContext(t.Context(), statusFunc)
	defer cancel()

	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected the context to be already canceled when not leading")
	}
	assert.ErrorIs(t, context.Cause(ctx), ErrNotLeading, "unexpected cancellation cause")
}

func TestLeaderContext_Leading_CancelsOnLeaseLost(t *testing.T) {
	expired := make(chan struct{})
	statusFunc := func() (Status, <-chan struct{}) { return StatusLeading, expired }

	ctx, cancel := LeaderContext(t.Context(), statusFunc)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("expected the context to still be active while leading")
	default:
	}

	close(expired)

	<-ctx.Done()
	assert.ErrorIs(t, context.Cause(ctx), ErrLostLead, "unexpected cancellation cause")
}

func TestLeaderContext_Leading_CancelsOnParentDone(t *testing.T) {
	statusFunc := func() (Status, <-chan struct{}) { return StatusLeading, make(chan struct{}) }

	parent, parentCancel := context.WithCancel(t.Context())
	ctx, cancel := LeaderContext(parent, statusFunc)
	defer cancel()

	parentCancel()

	<-ctx.Done()
	assert.ErrorIs(t, context.Cause(ctx), context.Canceled, "unexpected cancellation cause")
}

func TestLeaderContext_Stop_ReleasesTheContext(t *testing.T) {
	statusFunc := func() (Status, <-chan struct{}) { return StatusLeading, make(chan struct{}) }

	ctx, cancel := LeaderContext(t.Context(), statusFunc)
	cancel()

	<-ctx.Done()
	assert.ErrorIs(t, context.Cause(ctx), context.Canceled, "unexpected cancellation cause")
}
