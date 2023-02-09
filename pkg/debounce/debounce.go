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

// Package debounce provides functionality to "debounce" multiple events in a
// given interval and handle only the most recent one. For the debounce pattern,
// see https://reactivex.io/documentation/operators/debounce.html. The blog post
// that inspired this implementation can be found here:
// https://drailing.net/2018/01/debounce-function-for-golang/.
package debounce

import (
	"context"
	"time"
)

// Debouncer throttles the items read from its input. Debouncer does this by
// forwarding items read from its input to its callback, except that it drops
// items that are followed by newer items before a timeout expires.
type Debouncer[T any] struct {
	// Input is the input channel whose items shall be debounced.
	Input <-chan T

	// Timeout defines the time that must pass without seeing any new items
	// until the most recent item is forwarded to the callback.
	Timeout time.Duration

	// Filter controls which elements reset the time window and which elements
	// are are simply silently dropped. It may be nil, in which case no items
	// will be dropped.
	Filter func(item T) bool

	// Callback is the func that receives debounced items.
	Callback func(item T)
}

// Run polls the input for items, throttles them and forwards them to the
// callback. Run returns either without an error when the input is closed or
// with the context's error if the context is done, whichever happens first.
func (d *Debouncer[T]) Run(ctx context.Context) error {
	// Create a timer that is initially expired
	timer := time.NewTimer(0)
	defer timer.Stop()
	<-timer.C

	for pendingItem := (*T)(nil); ; {
		select {
		case <-ctx.Done():
			return ctx.Err() // ctx is done, good bye ...

		case item, ok := <-d.Input:
			if !ok {
				return nil // input channel closed, good bye ...
			}

			if d.Filter != nil && !d.Filter(item) {
				continue // the current item has been filtered out ...
			}

			// The current item needs to reset the timer.
			// There are three possible states:
			// https://stackoverflow.com/a/58631999
			if pendingItem == nil {
				// (1) The pendingItem has been consumed.
				// The timer must have expired and its tick must have been consumed.
			} else if timer.Stop() {
				// (2) The timer was stopped before it expired.
				// Its channel cannot hold a tick.
			} else {
				// (3) The timer has already expired, but the pendingItem has not been consumed yet.
				// The pending tick needs to be consumed.
				<-timer.C
			}

			timer.Reset(d.Timeout)

			// Save the current item to be sent after the timeout.
			pendingItem = &item

			// Now it is safe to restart the timer.
			timer.Reset(d.Timeout)

		case <-timer.C:
			d.Callback(*pendingItem)
			// Clear out the item: Indicates that the timer tick has been
			// consumed and allows the item to be GC'd.
			pendingItem = nil
		}
	}
}
