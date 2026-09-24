// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package secret

// A secret byte slice for values that need no kind of their own.
// Copying Bytes does not create an independent copy of the underlying data.
type Bytes struct {
	Value[Bytes, []byte]
}

// The [NoValueError] for the zero [Bytes].
type NoBytesError = NoValueError[Bytes]

// Wraps the given bytes as secret [Bytes] without copying the slice.
// Mutations through the original slice are visible through the returned Bytes.
func FromBytes(bytes []byte) Bytes { return Bytes{From[Bytes](bytes)} }
