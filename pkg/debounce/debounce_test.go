/*
Copyright 2020 k0s authors

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
	ctx, cancel := context.WithCancel(context.TODO())

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

	for i := 0; i < 1000; i++ {
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
	ctx, cancel := context.WithCancel(context.TODO())

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
	ctx, cancel := context.WithCancel(context.TODO())
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
