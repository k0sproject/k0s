// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"fmt"
	"time"

	"github.com/k0sproject/k0s/internal/sync/activity"
	"github.com/k0sproject/k0s/internal/sync/value"
)

// Debounces activity recorded by a [activity.Tracker].
func ExampleDebounce() {
	touch, last := activity.Tracker(time.Now())

	stop, settled := make(chan struct{}), make(chan struct{})
	defer close(stop)

	go activity.Debounce(stop, time.Millisecond, last, func(time.Time) {
		close(settled)
	})

	for range 3 {
		touch(time.Now())
	}

	<-settled
	fmt.Println("The activity has settled.")

	// Output: The activity has settled.
}

// Debounces activity stored in a [value.Latest].
func ExampleDebounce_latest() {
	var lastActivity value.Latest[time.Time]

	stop, settled := make(chan struct{}), make(chan struct{})
	defer close(stop)

	go activity.Debounce(stop, time.Millisecond, lastActivity.Peek, func(time.Time) {
		close(settled)
	})

	lastActivity.Set(time.Now())

	<-settled
	fmt.Println("The activity has settled.")

	// Output: The activity has settled.
}
