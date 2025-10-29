// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"testing"
	assert "github.com/stretchr/testify/assert"
)

func TestUpdateCheckIntervalIsEnabled(t *testing.T) {
	os.Setenv("K0S_UPDATE_CHECK_INTERVAL", "10s")
	defer os.Unsetenv("K0S_UPDATE_CHECK_INTERVAL")
	
	assert.Equal(t, "10s", os.Getenv("K0S_UPDATE_CHECK_INTERVAL"))
	assert.False(t, isCheckUpdatesDisabled())
}

func TestUpdateCheckIntervalIsEnbledByDefault(t *testing.T) {
	os.Unsetenv("K0S_UPDATE_CHECK_INTERVAL")
	assert.False(t, isCheckUpdatesDisabled())
}

func TestUpdateCheckIntervalIsDisabled(t *testing.T) {
	t.Run("lower case", func (t *testing.T) {
		os.Setenv("K0S_UPDATE_CHECK_INTERVAL", "disabled")
		defer os.Unsetenv("K0S_UPDATE_CHECK_INTERVAL")
		assert.True(t, isCheckUpdatesDisabled())
	})

	t.Run("mixed case", func (t *testing.T) {
		os.Setenv("K0S_UPDATE_CHECK_INTERVAL", "Disabled")
		defer os.Unsetenv("K0S_UPDATE_CHECK_INTERVAL")
		assert.True(t, isCheckUpdatesDisabled())
	})

	t.Run("upper case", func (t *testing.T) {
		os.Setenv("K0S_UPDATE_CHECK_INTERVAL", "DISABLED")
		defer os.Unsetenv("K0S_UPDATE_CHECK_INTERVAL")
		assert.True(t, isCheckUpdatesDisabled())
	})

	assert.False(t, isCheckUpdatesDisabled())
}
