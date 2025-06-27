// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package debounce

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebounce(t *testing.T) {
	const numEvents = 5
	eventChan := make(chan int32, numEvents)
	var debounceCalled uint32
	var lastItem int32
	ctx, cancel := context.WithCancel(t.Context())

	debouncer := Debouncer[int32]{
		Input:   eventChan,
		Timeout: 10 * time.Millisecond,
		Callback: func(item int32) {
			atomic.AddUint32(&debounceCalled, 1)
			atomic.StoreInt32(&lastItem, item)
		},
	}

	for i := int32(1); i <= numEvents; i++ {
		eventChan <- i
	}

	runReturned := make(chan error)
	go func() { runReturned <- debouncer.Run(ctx) }()

	for range 1000 {
		time.Sleep(10 * time.Millisecond)
		if atomic.LoadInt32(&lastItem) == numEvents {
			break
		}
	}

	cancel()

	select {
	case <-time.After(1 * time.Second):
		require.Fail(t, "Debouncer didn't terminate in time")
	case err := <-runReturned:
		assert.Same(t, context.Canceled, err)
	}

	assert.Equal(t, uint32(1), atomic.LoadUint32(&debounceCalled))
	assert.Equal(t, int32(numEvents), atomic.LoadInt32(&lastItem))
}

func TestDebounceStopWithoutActuallyDebouncing(t *testing.T) {
	const numEvents = 5
	eventChan := make(chan int, numEvents)
	var debounceCalled uint32
	ctx, cancel := context.WithCancel(t.Context())

	debouncer := Debouncer[int]{
		Input:    eventChan,
		Timeout:  10 * time.Second,
		Callback: func(int) { atomic.AddUint32(&debounceCalled, 1) },
	}

	for i := 1; i <= numEvents; i++ {
		eventChan <- i
	}

	runReturned := make(chan error)
	go func() { runReturned <- debouncer.Run(ctx) }()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-time.After(1 * time.Second):
		require.Fail(t, "Debouncer didn't terminate in time")
	case err := <-runReturned:
		assert.Same(t, context.Canceled, err)
	}

	assert.Equal(t, uint32(0), atomic.LoadUint32(&debounceCalled))

	eventChan <- -1
	sentinel := <-eventChan
	assert.Equal(t, -1, sentinel, "Debouncer didn't consume all events")
}

func TestDebouncerReturnsIfInputIsClosed(t *testing.T) {
	eventChan := make(chan int)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	debouncer := Debouncer[int]{
		Input:    eventChan,
		Timeout:  10 * time.Second,
		Callback: func(int) {},
	}

	runReturned := make(chan error)
	go func() { runReturned <- debouncer.Run(ctx) }()

	time.Sleep(10 * time.Millisecond)
	close(eventChan)

	select {
	case <-time.After(1 * time.Second):
		assert.Fail(t, "Debouncer didn't return in time")
	case err := <-runReturned:
		assert.NoError(t, err)
	}
}
