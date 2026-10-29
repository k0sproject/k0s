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
	type (
		Lease  = leasecounter.Lease
		Leases = leasecounter.Leases
		State  = leasecounter.KonnectivityState
	)

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

	leases := func(leases ...leasecounter.Lease) []leasecounter.Lease { return leases }

	a := Lease{Name: "k0s-ctrl-a", Active: true}
	b := Lease{Name: "k0s-ctrl-b", Active: true}
	c := Lease{Name: "k0s-ctrl-c", Active: true}
	three := Leases{ActiveCount: 3, All: leases(a, b, c)}
	two := Leases{ActiveCount: 2, All: leases(a, b)}
	one := Leases{ActiveCount: 1, All: leases(a)}

	t.Run("publishes nothing before the first update", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		count, changed := underTest.Peek()
		assert.Zero(t, count)
		assertChanged(t, false, changed)
	})

	t.Run("publishes the first count as is", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		_, changed := underTest.Peek()

		recorded := underTest.Update(three)

		assertChanged(t, true, changed)
		count, _ := underTest.Peek()
		assert.Equal(t, KonnectivityServerCount{Count: 3}, count)
		assert.Equal(t, &State{ServerCount: 3}, recorded, "Expected the count to be recorded without counting a rise")
	})

	t.Run("publishes a rise immediately", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		underTest.Update(two)
		_, changed := underTest.Peek()

		recorded := underTest.Update(three)

		assertChanged(t, true, changed)
		count, _ := underTest.Peek()
		assert.Equal(t, KonnectivityServerCount{Count: 3}, count)
		assert.Equal(t, &State{ServerCount: 3, ServerCountRises: 1}, recorded, "Expected a rise above the advertised count to be counted")
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

			recorded := underTest.Update(two)
			assertChanged(t, false, changed)
			assert.Equal(t, &State{ServerCount: 3}, recorded, "Expected the withheld drop not to be recorded")

			time.Sleep(time.Minute - time.Nanosecond)
			synctest.Wait()
			assertChanged(t, false, changed)
			count, _ := underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 3}, count, "Expected the drop to be withheld for the debounce period")

			time.Sleep(time.Nanosecond)
			synctest.Wait()
			assertChanged(t, true, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 2}, count, "Expected the drop to be published after the debounce period")
			assert.Equal(t, &State{ServerCount: 2}, underTest.Update(two), "Expected the published drop to be recorded")
		})
	})

	t.Run("publishes the latest drop after the debounce period without extending it", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			underTest.Update(two)

			time.Sleep(40 * time.Second)
			synctest.Wait()
			underTest.Update(one)

			time.Sleep(20*time.Second - time.Nanosecond)
			synctest.Wait()
			count, changed := underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 3}, count, "Expected another drop to be withheld for the same debounce period")

			time.Sleep(time.Nanosecond)
			synctest.Wait()
			assertChanged(t, true, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 1}, count, "Expected the latest drop to be published")
		})
	})

	t.Run("discards a drop that recovers within the debounce period", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			_, changed := underTest.Peek()

			underTest.Update(two)
			time.Sleep(time.Minute - time.Nanosecond)
			synctest.Wait()
			recorded := underTest.Update(three)

			assertChanged(t, false, changed)
			assert.Equal(t, &State{ServerCount: 3}, recorded, "Expected the discarded drop not to be recorded")

			// Let the debounce period elapse to ensure the drop isn't published belatedly.
			time.Sleep(time.Second)
			synctest.Wait()
			assertChanged(t, false, changed)
			count, _ := underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 3}, count, "Expected the drop to be discarded")
		})
	})

	t.Run("discards a pending drop on a rise", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			underTest.Update(two)

			underTest.Update(Leases{ActiveCount: 4, All: leases(a, b, c, Lease{Name: "k0s-ctrl-d", Active: true})})
			count, changed := underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 4}, count, "Expected the rise to be published immediately")

			// Let the debounce period elapse to ensure the drop isn't published belatedly.
			time.Sleep(time.Minute)
			synctest.Wait()
			assertChanged(t, false, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 4}, count, "Expected the pending drop to be discarded")
		})
	})

	t.Run("withholds further drops after a published one", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			underTest.Update(two)
			time.Sleep(time.Minute)
			synctest.Wait()
			count, changed := underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 2}, count)

			underTest.Update(one)
			time.Sleep(time.Minute - time.Nanosecond)
			synctest.Wait()
			assertChanged(t, false, changed)

			time.Sleep(time.Nanosecond)
			synctest.Wait()
			assertChanged(t, true, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 1}, count)
		})
	})

	t.Run("counts a rise after a published drop", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			own := Lease{Name: a.Name, Active: true, Konnectivity: &State{ServerCount: 3, ServerCountRises: 4}}
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(Leases{ActiveCount: 3, Own: &own, All: leases(own, b, c)})
			underTest.Update(Leases{ActiveCount: 2, Own: &own, All: leases(own, b)})
			time.Sleep(time.Minute)
			synctest.Wait()
			assert.Equal(t, &State{ServerCount: 2, ServerCountRises: 4}, underTest.Update(Leases{ActiveCount: 2, Own: &own, All: leases(own, b)}), "Expected the published drop to keep the server count rises")

			recorded := underTest.Update(Leases{ActiveCount: 3, Own: &own, All: leases(own, b, c)})

			assert.Equal(t, &State{ServerCount: 3, ServerCountRises: 5}, recorded, "Expected the rise to be counted and the count to be recorded")
		})
	})

	t.Run("counts a rise above the recorded count", func(t *testing.T) {
		// This happens if a drop was published before the controller was restarted.
		own := Lease{Name: a.Name, Active: true, Konnectivity: &State{ServerCount: 2, ServerCountRises: 4}}
		var underTest KonnectivityServerCountDebouncer

		recorded := underTest.Update(Leases{ActiveCount: 3, Own: &own, All: leases(own, b, c)})

		assert.Equal(t, &State{ServerCount: 3, ServerCountRises: 5}, recorded, "Expected the rise above the recorded count to be counted")
	})

	t.Run("doesn't count a rise to the recorded count", func(t *testing.T) {
		own := Lease{Name: a.Name, Konnectivity: &State{ServerCount: 3, ServerCountRises: 4}}
		var underTest KonnectivityServerCountDebouncer

		recorded := underTest.Update(Leases{ActiveCount: 3, Own: &own, All: leases(own, b, c)})

		assert.Equal(t, &State{ServerCount: 3, ServerCountRises: 4}, recorded, "Expected a restart without a prior drop to keep the server count rises")
	})

	t.Run("doesn't count a rise above an unknown recorded count", func(t *testing.T) {
		own := Lease{Name: a.Name, Active: true, Konnectivity: &State{ServerCount: 0, ServerCountRises: 4}}
		var underTest KonnectivityServerCountDebouncer

		recorded := underTest.Update(Leases{ActiveCount: 3, Own: &own, All: leases(own, b, c)})

		assert.Equal(t, &State{ServerCount: 3, ServerCountRises: 4}, recorded, "Expected a rise above an unknown recorded count not to be counted")
	})

	t.Run("keeps the counted rises until the lease catches up", func(t *testing.T) {
		own := Lease{Name: a.Name, Active: true, Konnectivity: &State{ServerCount: 2, ServerCountRises: 4}}
		var underTest KonnectivityServerCountDebouncer
		assert.Equal(t, &State{ServerCount: 3, ServerCountRises: 5}, underTest.Update(Leases{ActiveCount: 3, Own: &own, All: leases(own, b, c)}))

		// The lease still shows the old state, as the patch hasn't landed yet.
		recorded := underTest.Update(Leases{ActiveCount: 3, Own: &own, All: leases(own, b, c)})

		assert.Equal(t, &State{ServerCount: 3, ServerCountRises: 5}, recorded, "Expected the counted rises not to regress")
	})

	t.Run("publishes the sum of all server count rises", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		_, changed := underTest.Peek()

		underTest.Update(Leases{
			ActiveCount: 3,
			All: leases(
				Lease{Name: a.Name, Active: true, Konnectivity: &State{ServerCount: 3, ServerCountRises: 4}},
				Lease{Name: b.Name, Active: true, Konnectivity: &State{ServerCount: 3, ServerCountRises: 1}},
				c,
				Lease{Name: "k0s-ctrl-d", Konnectivity: &State{ServerCount: 2, ServerCountRises: 2}},
			),
		})

		assertChanged(t, true, changed)
		count, _ := underTest.Peek()
		assert.Equal(t, KonnectivityServerCount{Count: 3, Rises: 7}, count,
			"Expected the server count rises of inactive leases to be summed up as well",
		)
	})

	t.Run("publishes changed server count rises immediately", func(t *testing.T) {
		var underTest KonnectivityServerCountDebouncer
		underTest.Update(three)
		_, changed := underTest.Peek()

		underTest.Update(Leases{
			ActiveCount: 3,
			All: leases(
				Lease{Name: a.Name, Active: true, Konnectivity: &State{ServerCount: 3, ServerCountRises: 1}},
				b,
				c,
			),
		})

		assertChanged(t, true, changed)
		count, _ := underTest.Peek()
		assert.Equal(t, KonnectivityServerCount{Count: 3, Rises: 1}, count)
	})

	t.Run("publishes changed server count rises while a drop is pending", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var underTest KonnectivityServerCountDebouncer
			underTest.Update(three)
			underTest.Update(two)
			_, changed := underTest.Peek()

			underTest.Update(Leases{ActiveCount: 2,
				All: leases(
					Lease{Name: a.Name, Active: true, Konnectivity: &State{ServerCount: 3, ServerCountRises: 1}},
					b,
				),
			})

			assertChanged(t, true, changed)
			count, changed := underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 3, Rises: 1}, count, "Expected the server count rises to be published with the count still withheld")

			time.Sleep(time.Minute)
			synctest.Wait()
			assertChanged(t, true, changed)
			count, _ = underTest.Peek()
			assert.Equal(t, KonnectivityServerCount{Count: 2, Rises: 1}, count, "Expected the drop to be published with the current server count rises")
		})
	})
}
