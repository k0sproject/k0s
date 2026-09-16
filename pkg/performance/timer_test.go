// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package performance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckpoint_NotStarted(t *testing.T) {
	timer := NewTimer("test").Buffer()

	timer.Checkpoint("cp")

	if assert.Len(t, timer.buffer, 1) {
		assert.Error(t, timer.buffer[0].err)
	}
}

func TestCheckpoint_Started(t *testing.T) {
	timer := NewTimer("test").Buffer().Start()

	timer.Checkpoint("cp")

	if assert.Len(t, timer.buffer, 1) {
		assert.NoError(t, timer.buffer[0].err)
	}
}

func TestOutput_DrainsBuffer(t *testing.T) {
	timer := NewTimer("test").Buffer().Start()
	timer.Checkpoint("cp1")
	timer.Checkpoint("cp2")

	timer.Output()

	assert.Empty(t, timer.buffer)
}
