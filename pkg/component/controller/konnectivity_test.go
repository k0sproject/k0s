// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/component/controller/leasecounter"

	"github.com/stretchr/testify/assert"
)

func TestKonnectivityServerCountDebouncer(t *testing.T) {
	type Leases = leasecounter.Leases

	assertChanged := func(t *testing.T, expected bool, actual <-chan struct{}) {
		t.Helper()
		select {
		case <-actual:
			if !expected {
				assert.Fail(t, "Expected no change to be published")
			}
		default:
			if expected {
				assert.Fail(t, "Expected a change to be published")
			}
		}
	}

	three, two, one := Leases{ActiveCount: 3}, Leases{ActiveCount: 2}, Leases{ActiveCount: 1}

	t.Run("publishes nothing before the first update", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		count, changed := underTest.Peek()
		assert.Zero(t, count)
		assertChanged(t, false, changed)
	})

	t.Run("publishes the first count as is", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		_, changed := underTest.Peek()

		underTest.Update(three)

		assertChanged(t, true, changed)
		count, _ := underTest.Peek()
		assert.Equal(t, uint(3), count)
	})

	t.Run("publishes a rise immediately", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		underTest.Update(two)
		_, changed := underTest.Peek()

		underTest.Update(three)

		assertChanged(t, true, changed)
		count, _ := underTest.Peek()
		assert.Equal(t, uint(3), count)
	})

	t.Run("doesn't republish an unchanged count", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		underTest.Update(three)
		_, changed := underTest.Peek()

		underTest.Update(three)

		assertChanged(t, false, changed)
	})

	t.Run("withholds a drop for the debounce period", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			_, changed := underTest.Peek()

			underTest.Update(two)
			assertChanged(t, false, changed)

			synctest.Sleep(time.Minute - time.Nanosecond)
			assertChanged(t, false, changed)
			count, _ := underTest.Peek()
			assert.Equal(t, uint(3), count, "Expected the drop to be withheld for the debounce period")

			synctest.Sleep(time.Nanosecond)
			assertChanged(t, true, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, uint(2), count, "Expected the drop to be published after the debounce period")
		})
	})

	t.Run("publishes the latest drop after the debounce period without extending it", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			underTest.Update(two)

			synctest.Sleep(40 * time.Second)
			underTest.Update(one)

			synctest.Sleep(20*time.Second - time.Nanosecond)
			count, changed := underTest.Peek()
			assert.Equal(t, uint(3), count, "Expected another drop to be withheld for the same debounce period")

			synctest.Sleep(time.Nanosecond)
			assertChanged(t, true, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, uint(1), count, "Expected the latest drop to be published")
		})
	})

	t.Run("discards a drop that recovers within the debounce period", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			_, changed := underTest.Peek()

			underTest.Update(two)
			synctest.Sleep(time.Minute - time.Nanosecond)
			underTest.Update(three)

			assertChanged(t, false, changed)

			// Let the debounce period elapse to ensure the drop isn't published belatedly.
			synctest.Sleep(time.Second)
			assertChanged(t, false, changed)
			count, _ := underTest.Peek()
			assert.Equal(t, uint(3), count, "Expected the drop to be discarded")
		})
	})

	t.Run("discards a pending drop on a rise", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			underTest.Update(two)

			underTest.Update(Leases{ActiveCount: 4})
			count, changed := underTest.Peek()
			assert.Equal(t, uint(4), count, "Expected the rise to be published immediately")

			// Let the debounce period elapse to ensure the drop isn't published belatedly.
			synctest.Sleep(time.Minute)
			assertChanged(t, false, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, uint(4), count, "Expected the pending drop to be discarded")
		})
	})

	t.Run("withholds further drops after a published one", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			underTest.Update(two)
			synctest.Sleep(time.Minute)
			count, changed := underTest.Peek()
			assert.Equal(t, uint(2), count)

			underTest.Update(one)
			synctest.Sleep(time.Minute - time.Nanosecond)
			assertChanged(t, false, changed)

			synctest.Sleep(time.Nanosecond)
			assertChanged(t, true, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, uint(1), count)
		})
	})
}
