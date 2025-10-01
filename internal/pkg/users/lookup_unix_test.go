//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupUID(t *testing.T) {
	uid, err := LookupUID("root")
	if assert.NoError(t, err, "Failed to get UID for root user") {
		assert.Equal(t, 0, uid, "root's UID is not 0?")
	}

	uid, err = LookupUID("some-non-existing-user")
	if assert.Error(t, err, "Got a UID for some-non-existing-user?") {
		assert.ErrorIs(t, err, ErrNotExist)
		var exitErr *exec.ExitError
		assert.ErrorAs(t, err, &exitErr, "expected external `id` to return an error")
		assert.Equal(t, UnknownUID, uid)
	}
}
