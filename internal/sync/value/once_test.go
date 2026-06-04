// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package value_test

import (
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/k0sproject/k0s/internal/sync/value"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var underTest value.Once[int]

		assert.False(t, underTest.IsSet(), "A zero Once shouldn't be set")

		// Acquire some done channels concurrently.
		var dones [100]<-chan struct{}
		start := make(chan struct{})
		for i := range dones {
			go func() { <-start; dones[i] = underTest.Done() }()
		}
		synctest.Wait()
		close(start)
		synctest.Wait()

		// Check that those done channels are all the same.
		require.NotNil(t, dones[0])
		for _, done := range dones[1:] {
			assert.Equal(t, dones[0], done)
		}

		// Check that the value is still unset.
		assert.False(t, underTest.IsSet())

		// Queue some goroutine that waits for a value.
		var got int
		go func() { got = underTest.Await() }()

		synctest.Wait()
		assert.False(t, underTest.IsSet())
		assert.Zero(t, got)

		// Now try to set values in parallel. Only one value may win.
		var updates atomic.Uint32
		start = make(chan struct{})
		for i := range 99 {
			i++
			go func() {
				<-start
				if underTest.Set(i) {
					updates.Add(1)
				}
			}()
		}
		synctest.Wait()
		close(start)
		synctest.Wait()

		// Now assert that a value has been set, exactly once.
		assert.Positive(t, got)
		assert.LessOrEqual(t, got, 100)
		assert.True(t, underTest.IsSet())
		assert.EqualValues(t, 1, updates.Load())

		// Check that the done channel has been closed.
		select {
		case <-dones[0]:
		default:
			assert.Fail(t, "Done channel was not closed")
		}

		select {
		case <-underTest.Done():
		default:
			assert.Fail(t, "Done channel returned after Set was not closed")
		}

		assert.Equal(t, got, underTest.Await())
	})
}
