// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package secret_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/secret"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A secret blob, declared the way secret values are meant to be declared.
type blob struct {
	// The redaction test must be able to identify the payload in case it leaks
	// through formatting. When fmt reaches a Value via reflection, it ignores
	// Value's methods and prints the payload as it sees fit for the verb, so
	// the test looks for the payload spelled the way fmt spells it on that
	// path, under each verb. The payload is a byte slice because fmt treats
	// those specially, and its spelling can't be mistaken for the address that
	// the safe output legitimately contains.
	secret.Value[blob, []byte]
}

func TestValue(t *testing.T) {
	t.Run("reveals the value", func(t *testing.T) {
		b := blob{secret.From[blob]([]byte("c0nf1d3n714l"))}
		revealed, err := b.Reveal()
		require.NoError(t, err)
		assert.Equal(t, []byte("c0nf1d3n714l"), revealed)
	})

	t.Run("holds no value when zero", func(t *testing.T) {
		revealed, err := blob{}.Reveal()
		assert.Nil(t, revealed)
		assert.Equal(t, secret.NoValueError[blob]{}, err)
		assert.EqualError(t, err, "no secret_test.blob")
	})

	t.Run("stores a value", func(t *testing.T) {
		var b blob
		b.Store([]byte("c0nf1d3n714l"))
		assert.False(t, b.IsZero(), "Should no longer be zero after storing")
		revealed, err := b.Reveal()
		require.NoError(t, err)
		assert.Equal(t, []byte("c0nf1d3n714l"), revealed)

		before := b
		b.Store([]byte("pr1v473"))
		revealed, err = b.Reveal()
		require.NoError(t, err)
		assert.Equal(t, []byte("pr1v473"), revealed, "Storing should replace the value")
		revealed, err = before.Reveal()
		require.NoError(t, err)
		assert.Equal(t, []byte("c0nf1d3n714l"), revealed, "Copies taken before should be unaffected")
	})

	t.Run("reveals the value to a function", func(t *testing.T) {
		b := blob{secret.From[blob]([]byte("c0nf1d3n714l"))}
		length, err := b.Use(func(value []byte) (int, error) { return len(value), nil })
		require.NoError(t, err)
		assert.Equal(t, 12, length)

		length, err = blob{}.Use(func([]byte) (int, error) {
			assert.Fail(t, "Function should not be called for the zero value")
			return 42, nil
		})
		assert.Equal(t, secret.NoValueError[blob]{}, err)
		assert.Zero(t, length)
	})

	t.Run("is zero when unset", func(t *testing.T) {
		assert.True(t, blob{}.IsZero(), "Zero value should be zero")
		assert.False(t, blob{secret.From[blob]([]byte{})}.IsZero(), "Empty slice should still count as set")
		assert.False(t, blob{secret.From[blob, []byte](nil)}.IsZero(), "Nil slice should still count as set")
	})

	t.Run("is named after the embedding type", func(t *testing.T) {
		assert.Equal(t, "<secret_test.blob>", blob{}.String())
	})

	t.Run("is redacted when formatted", func(t *testing.T) {
		expectations := map[string]string{
			"%s":  "<secret_test.blob>",
			"%v":  "<secret_test.blob>",
			"%+v": "<secret_test.blob>",
			"%#v": "<secret_test.blob>",
			"%q":  `"<secret_test.blob>"`,
			"%x":  "%!x(secret_test.blob)",
			"%d":  "%!d(secret_test.blob)",
			"%c":  "%!c(secret_test.blob)",
		}
		unprotected := []byte("c0nf1d3n714l")
		// Build a set of all currently anticipated leaking representations.
		leaks := map[string]struct{}{
			"yzbuzjfkm243mtrs":         {}, // lowercase base64
			"63306e663164336e3731346c": {}, // lowercase base16
		}
		for verb := range expectations {
			leaks[strings.ToLower(fmt.Sprintf(verb, unprotected))] = struct{}{}
			// Formatting a reflect.Value takes fmt's reflection path, which produces different outcomes.
			leaks[strings.ToLower(fmt.Sprintf(verb, reflect.ValueOf(unprotected)))] = struct{}{}
		}

		underTest := blob{secret.From[blob](unprotected)}

		for verb, expected := range expectations {
			t.Run("embedding value as "+verb, func(t *testing.T) {
				assert.Equal(t, expected, fmt.Sprintf(verb, underTest))
			})
			t.Run("pointer to embedding value as "+verb, func(t *testing.T) {
				assert.Equal(t, expected, fmt.Sprintf(verb, &underTest))
			})
			t.Run("embedded value as "+verb, func(t *testing.T) {
				assert.Equal(t, expected, fmt.Sprintf(verb, underTest.Value))
			})
		}

		t.Run("embedding value as %p", func(t *testing.T) {
			formatted := fmt.Sprintf("%p", underTest)
			assert.Regexp(t, `%!p\(secret_test\.blob=\{\{0x[0-9a-f]{4,16}\}\}\)`, formatted)
		})

		// Containers have special formatting rules. Test all of them individually.
		for verb := range expectations {
			for _, tt := range []struct {
				name      string
				container any
			}{
				{"slice", []blob{underTest}},
				{"map", map[string]blob{"k": underTest}},
				{"interface", []any{underTest}},
				{"exported field", struct{ B blob }{underTest}},
				{"unexported field", struct{ b blob }{underTest}},
			} {
				t.Run(tt.name+" as "+verb, func(t *testing.T) {
					candidate := strings.ToLower(fmt.Sprintf(verb, tt.container))
					for leak := range leaks {
						assert.NotContainsf(t, candidate, leak, "Protected value has been revealed")
					}
				})
			}
		}
	})
}
