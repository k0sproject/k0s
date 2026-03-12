// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package value

import (
	"sync/atomic"
)

// Stores a value that can be set exactly once and awaited by others.
//
// The zero value is unset and ready to use.
type Once[T any] struct {
	p atomic.Pointer[val[T]]
}

// Reports whether a value has already been set.
func (f *Once[T]) IsSet() bool {
	if loaded := f.p.Load(); loaded != nil {
		select {
		case <-loaded.ch:
			return true
		default:
			return false
		}
	}

	return false
}

// Returns a channel that is closed once a value has been set.
func (f *Once[T]) Done() <-chan struct{} {
	return f.val().ch
}

// Await blocks until a value has been set and then returns it.
func (f *Once[T]) Await() T {
	val := f.val()
	<-f.val().ch
	return val.inner
}

func (f *Once[T]) val() *val[T] {
	loaded := f.p.Load()
	if loaded == nil {
		loaded = &val[T]{ch: make(chan struct{})}
		if !f.p.CompareAndSwap(nil, loaded) {
			loaded = f.p.Load()
		}
	}

	return loaded
}

// Set stores value if no value has been set yet.
//
// It returns true if this call stored the value, or false if another value had
// already been set before or during the call.
func (f *Once[T]) Set(value T) bool {
	for {
		loaded := f.p.Load()
		if loaded != nil {
			select {
			case <-loaded.ch:
				return false
			default:
			}
		}

		set := &val[T]{value, make(chan struct{})}
		close(set.ch)

		if f.p.CompareAndSwap(loaded, set) {
			if loaded != nil {
				loaded.inner = value
				close(loaded.ch)
			}
			return true
		}
	}
}
