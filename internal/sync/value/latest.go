/*
Copyright 2024 k0s authors

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

package value

import "sync/atomic"

// A value that can be atomically updated, where each update invalidates the
// previous value. Whenever the value changes, an associated expiration channel
// is closed. Callers can use this to be notified of updates. The zero value of
// Latest holds a zero value of T.
//
// Latest is useful when some shared state is updated frequently, and readers
// don't need to keep track of every value, just the latest one. Latest makes
// this easy, as there's no need to maintain a separate channel for each reader.
//
// Example Usage:
//
//	package main
//
//	import (
//		"fmt"
//		"sync"
//		"time"
//
//		"github.com/k0sproject/k0s/internal/sync/value"
//	)
//
//	func main() {
//		// Declare a zero latest value
//		var l value.Latest[int]
//
//		fmt.Println("Zero value:", l.Get()) // Output: Zero value: 0
//
//		// Set the value
//		l.Set(42)
//		fmt.Println("Value set to 42")
//
//		// Peek at the current value and get the expiration channel
//		value, expired := l.Peek()
//		fmt.Println("Peeked value:", value) // Output: Peeked value: 42
//
//		// Use a goroutine to watch for expiration
//		var wg sync.WaitGroup
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			<-expired
//			fmt.Println("Value expired, new value:", l.Get()) // Output: Value expired, new value: 84
//		}()
//
//		// Set a new value, which will expire the previous value
//		time.Sleep(1 * time.Second) // Simulate some delay
//		l.Set(84)
//		fmt.Println("New value set to 84")
//
//		wg.Wait() // Wait for the watcher goroutine to finish
//	}
type Latest[T any] struct {
	p atomic.Pointer[val[T]]
}

func NewLatest[T any](value T) *Latest[T] {
	latest := new(Latest[T])
	latest.Set(value)
	return latest
}

// Retrieves the latest value and its associated expiration channel. If no value
// was previously set, it returns the zero value of T and an expiration channel
// that is closed as soon as a value is set.
func (l *Latest[T]) Peek() (T, <-chan struct{}) {
	if loaded := l.p.Load(); loaded != nil {
		return loaded.inner, loaded.ch
	}

	value := val[T]{ch: make(chan struct{})}
	if !l.p.CompareAndSwap(nil, &value) {
		loaded := l.p.Load()
		return loaded.inner, loaded.ch
	}

	return value.inner, value.ch
}

// Sets a new value and closes the expiration channel of the previous value.
func (l *Latest[T]) Set(value T) {
	if old := l.p.Swap(&val[T]{value, make(chan struct{})}); old != nil {
		close(old.ch)
	}
}

type val[T any] struct {
	inner T
	ch    chan struct{}
}
