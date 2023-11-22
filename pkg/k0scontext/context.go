/*
Copyright 2023 k0s authors

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

// Package k0scontext provides utility functions for working with Go contexts
// and specific keys for storing and retrieving k0s project-related
// configuration data.
//
// The package also includes functions for setting, retrieving, and checking the
// presence of values associated with specific types. These functions simplify
// context-based data management in a type-safe and ergonomic manner. Each
// distinct type serves as a key for context values, removing the necessity for
// an extra key constant. The type itself becomes the key.
package k0scontext

import (
	"context"

	k0sapi "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// Key represents a string key used for storing and retrieving values in a context.
type Key string

// Context keys for storing k0s project configuration data in a context.
const (
	ContextNodeConfigKey    Key = "k0s_node_config"
	ContextClusterConfigKey Key = "k0s_cluster_config"
)

// FromContext retrieves a value from the context associated with the given key
// and attempts to cast it to the specified type. It returns the value or nil if
// not found.
func FromContext[out any](ctx context.Context, key Key) *out {
	v, ok := ctx.Value(key).(*out)
	if !ok {
		return nil
	}
	return v
}

// GetNodeConfig retrieves the k0s NodeConfig from the context, or nil if not found.
func GetNodeConfig(ctx context.Context) *k0sapi.ClusterConfig {
	cfg, ok := ctx.Value(ContextNodeConfigKey).(*k0sapi.ClusterConfig)
	if !ok {
		return nil
	}

	return cfg
}

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
