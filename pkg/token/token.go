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

package token

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/bits"
)

// A Kubernetes bootstrap token. Matches the regex [a-z0-9]{6}\.[a-z0-9]{16}.
// The first part is the "Token ID", the second part is the "Token Secret".
//
// https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/#token-format
type BootstrapToken [23]byte

func randomToken() (*BootstrapToken, error) {
	rng := func(b []byte) error { _, err := rand.Read(b); return err }
	tok, err := generate23Base36(rng)
	if err != nil {
		return nil, err
	}

	tok[6] = '.' // Set the 7th byte to a dot to match the token format.
	return (*BootstrapToken)(tok), nil
}

func (t *BootstrapToken) String() string {
	return fmt.Sprintf("%s.****************", t.ID())
}

func (t *BootstrapToken) ID() string {
	return string(t[:6])
}

func (t *BootstrapToken) secret() string {
	return string(t[7:])
}

func (t *BootstrapToken) token() string {
	return string(t[:])
}

// Generate a random 23-byte base36 encoded string using the provided random
// number generator.
func generate23Base36(rng func([]byte) error) (*[23]byte, error) {
	// Generate a 119-bit (2^118 < 36^23 < 2^119) random number represented as
	// two 64 bit digits by using 15 random bytes and clearing the most
	// significant bit of the most significant byte. The number is then is
	// converted to a base36 encoded string and stored in the provided buffer.

	var (
		hi   uint64   // High 55 bits of the 119-bit random number.
		lo   uint64   // Low 64 bits of the 119-bit random number.
		data [23]byte // Temporary and result buffer.
	)

	// Continuously generate random numbers until one fits within the desired
	// range [0, 36^23-1]. Each iteration has a ~93.8% probability of success.
	for {
		// Generate 15 random bytes (= 120 random bits).
		if err := rng(data[:15]); err != nil {
			return nil, err
		}

		// Zero out the most significant bit of the most significant byte to go
		// from 120 to 119 random bits.
		data[14] = data[14] & 0x7F

		// The upper bound of the range (36^23-1).
		const himax, lomax uint64 = 33809425810441975, 107593809847648255

		// Interpret the last 8 bytes as the high part and first 8 as the low part.
		hi = binary.LittleEndian.Uint64(data[8:])
		lo = binary.LittleEndian.Uint64(data[:8])

		// Check if the generated number is within the valid range.
		if hi < himax || (hi == himax && lo <= lomax) {
			break // The number is in range, proceed.
		}
	}

	// Convert the number to its base36 representation.
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	i := 22 // Fill from right to left.
	for hi > 0 || lo > 0 {
		var hiMod, mod uint64
		hi, hiMod = hi/36, hi%36
		lo, mod = bits.Div64(hiMod, lo, 36)
		data[i] = alphabet[mod]
		i--
	}

	// Left-pad the remaining digits with '0'.
	for ; i >= 0; i-- {
		data[i] = '0'
	}

	return &data, nil
}
