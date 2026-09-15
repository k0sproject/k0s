// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch_test

import (
	"os"
	"testing"

	"github.com/k0sproject/k0s/internal/os/fs/watch"
	"github.com/stretchr/testify/assert"
)

func TestGone(t *testing.T) {
	info, err := watch.PathGone.Stat()
	assert.Nil(t, info, "May never return a non-nil FileInfo")
	assert.Equal(t, watch.PathGone, err, "Must return itself as an error")
	assert.ErrorIs(t, err, os.ErrNotExist, "The error must indicate a non-existent path")
}
