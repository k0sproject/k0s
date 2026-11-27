// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/internal/sync/activity"
	"github.com/stretchr/testify/assert"
)

func TestDebounce(t *testing.T) {
	t.Run("no activity no signal", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				never := make(<-chan struct{})
				noEvent := func() (none time.Time, _ <-chan struct{}) {
					return none, never
				}

				activity.Debounce(t.Context().Done(), time.Minute, noEvent, func(activity time.Time) {
					assert.Failf(t, "Expected no debounces at all", "%s", activity)
				})
			}()

			t.Cleanup(func() {
				synctest.Wait()
				select {
				case <-done:
				default:
					assert.Fail(t, "Debounce still running")
				}
			})

			time.Sleep(time.Hour)
		})
	})

	t.Run("stop during quiet period", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			done := make(chan struct{})

			touch, last := activity.Tracker(time.Time{})
			go func() {
				defer close(done)
				activity.Debounce(t.Context().Done(), time.Minute, last, func(activity time.Time) {
					assert.Failf(t, "Expected no debounces at all", "%s", activity)
				})
			}()
			t.Cleanup(func() {
				synctest.Wait()
				select {
				case <-done:
				default:
					assert.Fail(t, "Debounce still running")
				}
			})

			touch(time.Now())
			time.Sleep(time.Second)
			synctest.Wait()
			select {
			case <-done:
				assert.Fail(t, "Debounce no longer running")
			default:
			}
		})
	})

	t.Run("single burst", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var debounces atomic.Pointer[[]time.Time]

			done := make(chan struct{})
			touch, last := activity.Tracker(time.Time{})
			go func() {
				defer close(done)
				var recorded []time.Time
				activity.Debounce(t.Context().Done(), time.Minute, last, func(activity time.Time) {
					recorded = append(recorded, activity)
					debounces.Store(new(slices.Clone(recorded)))
				})
			}()
			t.Cleanup(func() {
				synctest.Wait()
				select {
				case <-done:
				default:
					assert.Fail(t, "Debounce still running")
				}
			})

			lastTouch := time.Now()
			touch(lastTouch)

			time.Sleep(time.Minute - time.Millisecond)
			synctest.Wait()
			assert.Empty(t, deref(&debounces), "Expected no debounce while the quiet period is still running")

			time.Sleep(time.Millisecond)
			synctest.Wait()
			assert.Equal(t, []time.Time{lastTouch}, deref(&debounces), "Expected exactly one debounce")
		})
	})

	t.Run("activity extends quiet period", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var debounces atomic.Pointer[[]time.Time]

			done := make(chan struct{})
			touch, last := activity.Tracker(time.Time{})
			go func() {
				defer close(done)
				var recorded []time.Time
				activity.Debounce(t.Context().Done(), time.Minute, last, func(activity time.Time) {
					recorded = append(recorded, activity)
					debounces.Store(new(slices.Clone(recorded)))
				})
			}()
			t.Cleanup(func() {
				synctest.Wait()
				select {
				case <-done:
				default:
					assert.Fail(t, "Debounce still running")
				}
			})

			touch(time.Now())
			time.Sleep(30 * time.Second)
			lastTouch := time.Now()
			touch(lastTouch)

			time.Sleep(time.Minute - time.Millisecond)
			synctest.Wait()
			assert.Empty(t, deref(&debounces), "Expected the second touch to extend the quiet period")

			time.Sleep(time.Millisecond)
			synctest.Wait()
			assert.Equal(t, []time.Time{lastTouch}, deref(&debounces), "Expected exactly one debounce")
		})
	})

	t.Run("unconsumed debounces are skipped", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var debounces atomic.Pointer[[]time.Time]

			done := make(chan struct{})
			releaseCtx, release := context.WithCancel(t.Context())
			touch, last := activity.Tracker(time.Time{})
			go func() {
				defer close(done)
				var recorded []time.Time
				activity.Debounce(t.Context().Done(), time.Minute, last, func(activity time.Time) {
					recorded = append(recorded, activity)
					debounces.Store(new(slices.Clone(recorded)))
					// Block the debouncer, so we can emulate missing events.
					<-releaseCtx.Done()
				})
			}()
			t.Cleanup(func() {
				synctest.Wait()
				select {
				case <-done:
				default:
					assert.Fail(t, "Debounce still running")
				}
			})

			firstTouch := time.Now()
			touch(firstTouch)
			time.Sleep(time.Minute)
			synctest.Wait()
			assert.Len(t, deref(&debounces), 1, "Expected only the first blocking debounce")

			lastTouch := firstTouch
			for round := range 3 {
				time.Sleep(time.Minute)
				lastTouch = time.Now()
				touch(lastTouch)
				synctest.Wait()
				assert.Equal(t, []time.Time{firstTouch}, deref(&debounces), "Expected still only the first blocking debounce (round %d)", round+1)
			}

			release()
			synctest.Wait()
			assert.Equal(t, []time.Time{firstTouch}, deref(&debounces), "Expected still only the first blocking debounce")

			time.Sleep(time.Minute)
			synctest.Wait()
			assert.Equal(t, []time.Time{firstTouch, lastTouch}, deref(&debounces), "Expected unconsumed debounces to be skipped")
		})
	})
}

func deref[T any](p *atomic.Pointer[T]) (val T) {
	if loaded := p.Load(); loaded != nil {
		val = *loaded
	}
	return
}
