// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package secret

// A secret string for values that need no kind of their own.
type String struct {
	Value[String, string]
}

// The [NoValueError] for the zero [String].
type NoStringError = NoValueError[String]

// Wraps the given string as a secret [String].
func FromString(value string) String { return String{From[String](value)} }
