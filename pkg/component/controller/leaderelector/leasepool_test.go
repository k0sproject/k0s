// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeasePool_YieldSuspendsAndResumes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		underTest, client := newLeasePoolForTest(t, 1)

		// Start the lease pool.
		require.NoError(t, underTest.Start(t.Context()))
		t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
		synctest.Wait()
		firstRun := client.nextRun()
		require.NotNil(t, firstRun, "Client hasn't been started")
		require.NoError(t, context.Cause(firstRun), "First client run finished unexpectedly early")

		// Yield!
		underTest.YieldLease()
		synctest.Wait()
		require.ErrorContains(t, context.Cause(firstRun), "lease pool is yielding")

		// Ensure the yield duration is respected.
		time.Sleep(30*time.Second - time.Nanosecond)
		synctest.Wait()
		require.Nil(t, client.nextRun(), "Client has been restarted during yield period")

		// Ensure a second yield will extend the yield period.
		underTest.YieldLease()
		time.Sleep(30*time.Second - time.Nanosecond)
		synctest.Wait()
		require.Nil(t, client.nextRun(), "Client has been restarted during extended yield period")
		time.Sleep(time.Nanosecond) // Advance time beyond the yield duration.
		synctest.Wait()

		// Check that the client has been restarted.
		secondRun := client.nextRun()
		require.NotNil(t, secondRun, "Client hasn't been restarted after yielding")
		require.NotSame(t, firstRun, secondRun)
		require.NoError(t, context.Cause(secondRun), "Second client run finished unexpectedly early")

		// Stop it again.
		require.NoError(t, underTest.Stop())
		require.ErrorContains(t, context.Cause(secondRun), "lease pool is stopping")
	})
}

func TestLeasePool_StopWhileYielding(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		underTest, client := newLeasePoolForTest(t, 1)

		// Start the lease pool.
		require.NoError(t, underTest.Start(t.Context()))
		t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
		synctest.Wait()
		firstRun := client.nextRun()
		require.NotNil(t, firstRun, "Client hasn't been started")
		require.NoError(t, context.Cause(firstRun), "First client run finished unexpectedly early")

		// Yield!
		underTest.YieldLease()
		synctest.Wait()
		require.ErrorContains(t, context.Cause(firstRun), "lease pool is yielding")

		// Stop it while yielded.
		require.NoError(t, underTest.Stop())
		synctest.Wait()
		require.Nil(t, client.nextRun(), "Client has been restarted unexpectedly")
	})
}

func newLeasePoolForTest(t *testing.T, bufferSize uint) (*LeasePool, *fakeLeaseClient) {
	nullLogger := logrus.New()
	nullLogger.SetOutput(io.Discard)

	client := fakeLeaseClient{t, make(chan context.Context, bufferSize)}
	return &LeasePool{log: logrus.NewEntry(nullLogger), client: &client}, &client
}

type fakeLeaseClient struct {
	t    *testing.T
	runs chan context.Context
}

func (f *fakeLeaseClient) Run(ctx context.Context, changed func(leaderelection.Status)) {
	select {
	case f.runs <- ctx:
	default:
		assert.Fail(f.t, "channel is full")
	}

	changed(leaderelection.StatusLeading)
	<-ctx.Done()
	changed(leaderelection.StatusPending)
}

func (f *fakeLeaseClient) nextRun() context.Context {
	f.t.Helper()
	select {
	case ctx := <-f.runs:
		require.NotNil(f.t, ctx, "Empty context in lease pool run")
		return ctx
	default:
		return nil
	}
}
