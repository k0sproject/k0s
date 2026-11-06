// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package secret

import (
	"os"
)

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

// Reads the named file into secret [Bytes],
// the way [os.ReadFile] would read it into a plain byte slice.
func ReadFile(name string) (b Bytes, err error) {
	raw, err := os.ReadFile(name)
	if err == nil {
		b.Store(raw)
	}
	return
}
