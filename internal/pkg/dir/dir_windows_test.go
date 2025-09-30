// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package dir_test

import (
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/dir"

	"github.com/stretchr/testify/require"
)

func TestPathListJoin(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		require.Empty(t, dir.PathListJoin())
	})
	t.Run("single", func(t *testing.T) {
		require.Equal(t, "foo", dir.PathListJoin("foo"))
	})
	t.Run("multiple", func(t *testing.T) {
		require.Equal(t, "foo;bar;baz", dir.PathListJoin("foo", "bar", "baz"))
	})
}
