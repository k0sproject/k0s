// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package k0scontext provides various utilities for working with Go contexts.
//
// This package also includes functions for setting, retrieving, and checking
// the presence of values associated with specific types. These functions
// simplify context-based data management in a type-safe and ergonomic way. Each
// distinct type serves as a key for context values, so there is no need for
// additional key constants. The type itself is the key.
package k0scontext

import (
	"context"
)

// keyType is used to create unique keys based on the types of the values stored in a context.
type keyType[T any] struct{}

// bucket is used to wrap values before storing them in a context.
type bucket[T any] struct{ inner T }

// WithValue adds a value of type T to the context and returns a new context with the added value.
func WithValue[T any](ctx context.Context, value T) context.Context {
	var key keyType[T]
	return context.WithValue(ctx, key, bucket[T]{value})
}

// HasValue checks if a value of type T is present in the context.
func HasValue[T any](ctx context.Context) bool {
	_, ok := value[T](ctx)
	return ok
}

// Value retrieves the value of type T from the context.
// If there's no such value, it returns the zero value of type T.
func Value[T any](ctx context.Context) T {
	return ValueOrElse[T](ctx, func() (_ T) { return })
}

// ValueOr retrieves the value of type T from the context.
// If there's no such value, it returns the specified fallback value.
func ValueOr[T any](ctx context.Context, fallback T) T {
	return ValueOrElse[T](ctx, func() T { return fallback })
}

// ValueOrElse retrieves the value of type T from the context.
// If there's no such value, it invokes the fallback function and returns its result.
func ValueOrElse[T any](ctx context.Context, fallbackFn func() T) T {
	if val, ok := value[T](ctx); ok {
		return val.inner
	}

	return fallbackFn()
}

// value retrieves a value of type T from the context along with a boolean
// indicating its presence.
func value[T any](ctx context.Context) (bucket[T], bool) {
	var key keyType[T]
	val, ok := ctx.Value(key).(bucket[T])
	return val, ok
}
