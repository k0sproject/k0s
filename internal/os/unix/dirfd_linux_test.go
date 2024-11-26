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

package unix_test

import (
	"testing"

	osunix "github.com/k0sproject/k0s/internal/os/unix"
	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathFD_StatSelf(t *testing.T) {
	dirPath := t.TempDir()

	p, err := osunix.OpenDir(dirPath, unix.O_PATH)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, p.Close()) })

	// An O_PATH descriptor cannot read anything.
	_, err = p.Readdirnames(1)
	assert.ErrorIs(t, err, unix.EBADF)

	// Verify that the fstatat syscall works for O_PATH file descriptors.
	// It's not documented in the Linux man pages, just fstat is.
	// See open(2).
	stat, err := p.StatSelf()
	if assert.NoError(t, err) {
		assert.True(t, stat.IsDir())
	}
}
