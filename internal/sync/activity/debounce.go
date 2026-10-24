// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package activity

import (
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
)

var _ = (*value.Latest[struct{}])(nil) // for godoc links

// Debounces activity into invocations of f. Blocks until stop is closed,
// invoking f with the most recent activity after each quietPeriod during which
// no newer activity was observed.
//
// The last function provides the activity to observe. It reports the most
// recent activity, plus a channel that becomes readable when newer activity may
// be available. Both [Tracker]'s returned "last" function and
// [value.Latest.Peek] (on a Latest[time.Time]) satisfy this contract.
// Activities that don't advance in time are ignored.
func Debounce(
	stop <-chan struct{},
	quietPeriod time.Duration,
	last func() (time.Time, <-chan struct{}),
	f func(time.Time),
) {
	var signaledActivity time.Time

	for {
		var activity time.Time
		{
			var changed <-chan struct{}
			activity, changed = last()
			if !activity.After(signaledActivity) {
				select {
				case <-changed:
					continue
				case <-stop:
					return
				}
			}
		}

		deadline := activity.Add(quietPeriod)
		if deadline.After(time.Now()) {
			select {
			case <-time.After(time.Until(deadline)):
				continue
			case <-stop:
				return
			}
		}

		signaledActivity = activity
		f(activity)
	}
}

// Tracks the most recent activity, starting out with the given one.
//
// The returned touch function records the given time as new activity. It's safe
// to be called from any goroutine. Touches that don't advance in time are
// ignored.
//
// The returned last function reports the most recent activity, plus a channel
// that becomes readable whenever newer activity gets recorded. It's safe to be
// called from any goroutine, but is optimized in a way that only a single
// consumer may wait for new activity at any time. For multiple consumers, the
// general purpose [value.Latest] can be used instead.
//
// See [Debounce] for a usage example.
func Tracker(activity time.Time) (touch func(time.Time), last func() (time.Time, <-chan struct{})) {
	var mu sync.Mutex
	changed := make(chan struct{}, 1)

	last = func() (time.Time, <-chan struct{}) {
		mu.Lock()
		activity := activity
		mu.Unlock()
		return activity, changed
	}

	touch = func(t time.Time) {
		mu.Lock()
		set := t.After(activity)
		if set {
			activity = t
		}
		mu.Unlock()

		// Send the wakeup strictly after the activity has been stored. The
		// goroutine re-reads the last activity after every wakeup. This way,
		// dropped wakeups are okay.
		if set {
			select {
			case changed <- struct{}{}:
			default:
			}
		}
	}

	return
}
