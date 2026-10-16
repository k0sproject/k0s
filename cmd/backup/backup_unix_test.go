//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupCmd_AcceptsConfigFlag(t *testing.T) {
	flag := NewBackupCmd().Flags().Lookup("config")
	require.NotNil(t, flag, "k0s backup must accept --config")
	assert.Equal(t, "c", flag.Shorthand)
}

func TestBackupCmd_RejectsConfigFromStdin(t *testing.T) {
	cmd := NewBackupCmd()
	cmd.SetArgs([]string{"--config=-", "--save-path=-"})
	cmd.SetIn(strings.NewReader("spec: {}\n"))
	cmd.SilenceUsage, cmd.SilenceErrors = true, true

	err := cmd.Execute()
	assert.ErrorContains(t, err, "cannot read the configuration from stdin")
}
