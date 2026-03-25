//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDedupePaths(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "single path",
			input:    []string{`C:\foo`},
			expected: []string{`C:\foo`},
		},
		{
			name:     "multiple unique paths",
			input:    []string{`C:\foo`, `C:\bar`, `C:\baz`},
			expected: []string{`C:\foo`, `C:\bar`, `C:\baz`},
		},
		{
			name:     "duplicate paths same case",
			input:    []string{`C:\foo`, `C:\foo`},
			expected: []string{`C:\foo`},
		},
		{
			name:     "duplicate paths different case",
			input:    []string{`C:\Foo`, `C:\foo`},
			expected: []string{`C:\Foo`},
		},
		{
			name:     "empty string filtered",
			input:    []string{`C:\foo`, "", `C:\bar`},
			expected: []string{`C:\foo`, `C:\bar`},
		},
		{
			name:     "dot path filtered",
			input:    []string{".", `C:\foo`},
			expected: []string{`C:\foo`},
		},
		{
			name:     "paths that clean to same value",
			input:    []string{`C:\foo\bar\..`, `C:\foo`},
			expected: []string{`C:\foo`},
		},
		{
			name:     "all empty or dot paths",
			input:    []string{"", ".", ""},
			expected: nil,
		},
		{
			name:     "preserves order of first occurrence",
			input:    []string{`C:\first`, `C:\second`, `C:\FIRST`, `C:\third`},
			expected: []string{`C:\first`, `C:\second`, `C:\third`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupePaths(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
