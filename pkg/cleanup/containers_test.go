//go:build linux || windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingRuntime hangs stop and remove until the context is cancelled.
type blockingRuntime struct {
	pods        []string
	listCalls   int
	stopCalls   int
	removeCalls int
}

func (r *blockingRuntime) Ping(context.Context) error { return nil }

func (r *blockingRuntime) ListContainers(context.Context) ([]string, error) {
	r.listCalls++
	// first list has pods, later lists are empty so the final check passes
	if r.listCalls == 1 {
		return r.pods, nil
	}
	return nil, nil
}

func (r *blockingRuntime) StopContainer(ctx context.Context, _ string) error {
	r.stopCalls++
	<-ctx.Done()
	return fmt.Errorf("failed to stop pod sandbox: %w", ctx.Err())
}

func (r *blockingRuntime) RemoveContainer(ctx context.Context, _ string) error {
	r.removeCalls++
	<-ctx.Done()
	return ctx.Err()
}

// a wedged CRI must not hang reset
func TestContainers_stopAllContainers_boundedByTimeout(t *testing.T) {
	rt := &blockingRuntime{pods: []string{"sandbox-a", "sandbox-b"}}
	c := &containers{
		containerRuntime: rt,
		stopTimeout:      50 * time.Millisecond,
		cleanupMounts:    func() error { return nil },
	}

	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- c.stopAllContainers() }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("stopAllContainers did not return, reset would hang")
	}

	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, 2, rt.stopCalls)
	assert.Equal(t, 2, rt.removeCalls)
}

func TestIsExpectedStopError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"real error", errors.New("boom"), false},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, true},
		{"wrapped deadline", fmt.Errorf("failed to stop pod sandbox: %w", context.DeadlineExceeded), true},
		{"grpc deadline string", errors.New("rpc error: code = DeadlineExceeded desc = context deadline exceeded"), true},
		{"apiserver gone", errors.New("dial tcp 10.96.0.1:443: connect: connection refused"), true},
		{"apiserver too slow", errors.New("error getting ClusterInformation: the server was unable to return a response in the time allotted, but may still be processing the request"), true},
		{"unrelated refused", errors.New("dial tcp 127.0.0.1:2379: connect: connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isExpectedStopError(tt.err))
		})
	}
}
