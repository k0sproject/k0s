// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package secret

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
)

// Holds a secret value of type T in a way that makes it hard to use it
// unconsciously in an unsafe context. The value is obtained via [Value.Reveal]
// or [Value.Use], and only via those. The zero value holds no secret value at
// all. [Value.IsZero] returns true, and both [Value.Reveal] and [Value.Use]
// fail for it. Formatting it never reveals the value, regardless of the verb.
//
// Value is meant to be embedded. K is the embedding type, which lends its name
// to the placeholder, the bad verb marker and the zero value's error:
//
//	type Token struct {
//	    secret.Value[Token, string]
//	}
//
// K must be a named type for the name to be meaningful.
//
// Value uses ordinary Go assignment semantics. It doesn't deep-copy referenced
// data. Slices, maps and pointers therefore retain aliases to their underlying
// data, including across copies of Value.
type Value[K, T any] struct {
	// The function to call to obtain the protected value. Nothing that prints
	// values by walking them with reflection can call it without resorting to
	// unsafe. This includes the fmt package, as well as any debug printer that
	// does so by design. The most any of them can get at is a code address.
	reveal func() T
}

// Wraps the given value as a secret [Value] of kind K.
func From[K, T any](value T) Value[K, T] { return Value[K, T]{func() T { return value }} }

// Indicates whether this is the zero value, which holds no secret value at all.
func (v Value[K, T]) IsZero() bool { return v.reveal == nil }

// Reveals the secret value. It fails with a [NoValueError] for the zero value,
// which has nothing to reveal, and doesn't fail otherwise, so that an optional
// secret can be used where it's set and skipped where it's not:
//
//	if value, err := s.Reveal(); err == nil {
//	    use(value)
//	}
func (v Value[K, T]) Reveal() (T, error) {
	if v.reveal == nil {
		var zero T
		return zero, NoValueError[K]{}
	}
	return v.reveal(), nil
}

// Reveals the secret value to the given function and returns whatever that
// returns. It fails with a [NoValueError] for the zero value, just like
// [Value.Reveal], and doesn't call the function in that case.
//
// Use transforms secret values and helps to confine access to the unprotected
// value to just the function call. However, it cannot enforce that scope. The
// given function can retain the value or expose it through its result or error.
// Prefer it over [Value.Reveal] whenever the latter would leave the unprotected
// value around for longer than its actual use.
func (v Value[K, T]) Use[U any](f func(T) (U, error)) (U, error) {
	if v.reveal == nil {
		var zero U
		return zero, NoValueError[K]{}
	}
	return f(v.reveal())
}

// Stores the given value as the secret value, replacing whatever was stored
// before. The value is obtained via [Value.Reveal] or [Value.Use], and only via
// those.
func (v *Value[K, T]) Store(value T) { v.reveal = func() T { return value } }

// Indicates that a zero secret value of kind K, which holds nothing, was to be
// revealed.
type NoValueError[K any] struct {
	// No state, since K says all there is to say.
}

// Error implements [error].
func (NoValueError[K]) Error() string { return "no " + nameOf[K]() }

// String implements [fmt.Stringer]. It never reveals the value.
func (Value[K, T]) String() string { return "<" + nameOf[K]() + ">" }

// Format implements [fmt.Formatter]. It never reveals the value, whatever the
// verb: the string verbs yield the placeholder, all others yield the usual bad
// verb marker, sans the value.
func (v Value[K, T]) Format(f fmt.State, verb rune) {
	switch verb {
	case 's', 'v':
		_, _ = io.WriteString(f, v.String())
	case 'q':
		_, _ = io.WriteString(f, strconv.Quote(v.String()))
	default:
		_, _ = io.WriteString(f, "%!"+string(verb)+"("+nameOf[K]()+")")
	}
}

// Returns the name of the given type, as %T would print it.
func nameOf[K any]() string { return reflect.TypeFor[K]().String() }
