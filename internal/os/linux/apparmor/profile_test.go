//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package apparmor_test

import (
	"testing"

	"github.com/k0sproject/k0s/internal/os/linux/apparmor"

	"github.com/stretchr/testify/assert"
)

func TestValidateProfileName(t *testing.T) {
	t.Run("accepts letters, digits, dots, underscores and dashes", func(t *testing.T) {
		for _, name := range []string{"k0s-containerd", "cri-containerd.apparmor.d", "my_profile", "9lives", "X"} {
			assert.NoErrorf(t, apparmor.ValidateProfileName(name), "While validating %q", name)
		}
	})

	t.Run("rejects empty names", func(t *testing.T) {
		assert.ErrorContains(t, apparmor.ValidateProfileName(""), "empty")
	})

	t.Run("rejects punctuation that is special in policy or labels", func(t *testing.T) {
		for _, name := range []string{"-leading", ".leading", "with space", "a//b", "a&b", ":ns:a", "/usr/bin/foo", "glob*", "@{var}", "a{b}", `"quoted"`} {
			assert.ErrorContainsf(t, apparmor.ValidateProfileName(name), "invalid character", "While validating %q", name)
		}
	})
}

func TestValidateProfileReference(t *testing.T) {
	t.Run("accepts paths, child profiles and namespaces", func(t *testing.T) {
		for _, name := range []string{"k0s-containerd", "/usr/sbin/containerd", "parent//child", ":ns:name", "@{var}"} {
			assert.NoErrorf(t, apparmor.ValidateProfileReference(name), "While validating %q", name)
		}
	})

	t.Run("rejects stacks of profiles", func(t *testing.T) {
		for _, name := range []string{"a//&b", "a//&b//&c", ":ns:a//&b"} {
			assert.ErrorContainsf(t, apparmor.ValidateProfileReference(name), "stacks of profiles", "While validating %q", name)
		}
	})

	t.Run("rejects empty names", func(t *testing.T) {
		assert.ErrorContains(t, apparmor.ValidateProfileReference(""), "empty")
	})

	t.Run("rejects whitespace and control characters", func(t *testing.T) {
		for _, name := range []string{"with space", "tab\tbed", "new\nline", "\x7fdel", " leading"} {
			assert.ErrorContainsf(t, apparmor.ValidateProfileReference(name), "invalid character", "While validating %q", name)
		}
	})
}
