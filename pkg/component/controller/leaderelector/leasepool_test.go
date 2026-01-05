// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeasePool_YieldSuspendsAndResumes(t *testing.T) {
	underTest, client := newLeasePoolForTest(t, 1)

	// Start the lease pool.
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	firstRun := client.nextRun()
	require.NoError(t, context.Cause(firstRun.ctx), "First client run finished unexpectedly early")

	// Yield!
	startYieldTime := time.Now()
	underTest.yieldLease(100 * time.Millisecond)
	require.ErrorContains(t, waitForContextDone(t, firstRun.ctx, time.Second), "lease pool is yielding")

	// Ensure the yield duration is respected.
	client.assertNoRun(50 * time.Millisecond)
	midYieldTime := time.Now()

	// Ensure a second yield will extend the yield period.
	underTest.yieldLease(100 * time.Millisecond)
	require.Less(t, time.Since(startYieldTime), 100*time.Millisecond, "Test execution was too slow, please retry")

	// Check that the client has been restarted.
	secondRun := client.nextRun()

	require.NotSame(t, firstRun.ctx, secondRun.ctx)
	require.NoError(t, context.Cause(secondRun.ctx), "Second client run finished unexpectedly early")
	assert.GreaterOrEqual(t, secondRun.time.Sub(midYieldTime), 100*time.Millisecond, "Second run started too early")

	// Stop it again.
	require.NoError(t, underTest.Stop())
	require.ErrorContains(t, waitForContextDone(t, secondRun.ctx, time.Second), "lease pool is stopping")
}

func TestLeasePool_StopWhileYielding(t *testing.T) {
	underTest, client := newLeasePoolForTest(t, 1)

	// Start the lease pool.
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	firstRun := client.nextRun()
	require.NoError(t, context.Cause(firstRun.ctx), "First client run finished unexpectedly early")

	// Yield!
	underTest.yieldLease(time.Hour)
	require.ErrorContains(t, waitForContextDone(t, firstRun.ctx, time.Second), "lease pool is yielding")

	// Stop it while yielded.
	require.NoError(t, underTest.Stop())
	client.assertNoRun(time.Millisecond)
}

func newLeasePoolForTest(t *testing.T, bufferSize uint) (*LeasePool, *fakeLeaseClient) {
	nullLogger := logrus.New()
	nullLogger.SetOutput(io.Discard)

	client := fakeLeaseClient{t, make(chan run, bufferSize)}
	return &LeasePool{log: logrus.NewEntry(nullLogger), client: &client}, &client
}

type run = struct {
	ctx  context.Context
	time time.Time
}

type fakeLeaseClient struct {
	t    *testing.T
	runs chan run
}

func (f *fakeLeaseClient) Run(ctx context.Context, changed func(leaderelection.Status)) {
	select {
	case f.runs <- run{ctx, time.Now()}:
	default:
		assert.Fail(f.t, "channel is full")
	}

	changed(leaderelection.StatusLeading)
	<-ctx.Done()
	changed(leaderelection.StatusPending)
}

func (f *fakeLeaseClient) nextRun() run {
	f.t.Helper()
	select {
	case run := <-f.runs:
		require.NotNil(f.t, run.ctx, "Empty context in lease pool run")
		return run
	case <-time.After(time.Second):
		require.FailNow(f.t, "Timed out waiting for next lease pool run")
		return run{}
	}
}

func (f *fakeLeaseClient) assertNoRun(duration time.Duration) {
	f.t.Helper()
	select {
	case run := <-f.runs:
		require.FailNowf(f.t, "Client has been restarted unexpectedly", "%v", run)
	case <-time.After(duration):
	}
}

func waitForContextDone(t *testing.T, ctx context.Context, timeout time.Duration) error {
	t.Helper()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-time.After(timeout):
		require.FailNow(t, "Timed out waiting for context to finish")
		return nil
	}
}
