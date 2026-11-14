// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckSingleTokenSource(t *testing.T) {
	testToken := "test-token-data"

	t.Run("returns nil when no token sources provided", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		err := CheckSingleTokenSource("", "")
		require.NoError(t, err)
	})

	t.Run("returns nil when only arg provided", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		err := CheckSingleTokenSource(testToken, "")
		require.NoError(t, err)
	})

	t.Run("returns nil when only file provided", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		err := CheckSingleTokenSource("", "/path/to/token")
		require.NoError(t, err)
	})

	t.Run("returns nil when only env provided", func(t *testing.T) {
		t.Setenv(EnvVarToken, testToken)

		err := CheckSingleTokenSource("", "")
		require.NoError(t, err)
	})

	t.Run("returns error when multiple token sources provided - env and arg", func(t *testing.T) {
		t.Setenv(EnvVarToken, testToken)

		err := CheckSingleTokenSource(testToken, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "you can only pass one token source")
		assert.Contains(t, err.Error(), EnvVarToken)
	})

	t.Run("returns error when multiple token sources provided - env and file", func(t *testing.T) {
		t.Setenv(EnvVarToken, testToken)

		err := CheckSingleTokenSource("", "/path/to/token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "you can only pass one token source")
	})

	t.Run("returns error when multiple token sources provided - arg and file", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		err := CheckSingleTokenSource(testToken, "/path/to/token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "you can only pass one token source")
	})

	t.Run("returns error when all three token sources provided", func(t *testing.T) {
		t.Setenv(EnvVarToken, testToken)

		err := CheckSingleTokenSource(testToken, "/path/to/token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "you can only pass one token source")
	})
}

func TestWarnDeprecatedTokenArg(t *testing.T) {
	std := logrus.StandardLogger()
	origOut := std.Out
	origHooks := std.ReplaceHooks(logrus.LevelHooks{})
	std.SetOutput(io.Discard)
	t.Cleanup(func() {
		std.SetOutput(origOut)
		std.ReplaceHooks(origHooks)
	})

	hook := test.NewGlobal()

	t.Run("warns when token arg is set", func(t *testing.T) {
		hook.Reset()

		WarnDeprecatedTokenArg("test-token-data")

		require.Len(t, hook.Entries, 1)
		assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
		assert.Equal(t, tokenArgDeprecationWarning, hook.LastEntry().Message)
	})

	t.Run("silent when token arg is empty", func(t *testing.T) {
		hook.Reset()

		WarnDeprecatedTokenArg("")

		assert.Empty(t, hook.Entries)
	})
}

func TestGetTokenData_EnvVar(t *testing.T) {
	testToken := "test-token-data"

	t.Run("reads token from K0S_TOKEN env var", func(t *testing.T) {
		t.Setenv(EnvVarToken, testToken)

		token, err := GetTokenData("", "")
		require.NoError(t, err)
		assert.Equal(t, testToken, token)
	})

	t.Run("empty K0S_TOKEN returns empty string", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		token, err := GetTokenData("", "")
		require.NoError(t, err)
		assert.Empty(t, token)
	})
}

func TestGetTokenData_TokenArg(t *testing.T) {
	testToken := "test-token-data"

	t.Run("reads token from argument", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		token, err := GetTokenData(testToken, "")
		require.NoError(t, err)
		assert.Equal(t, testToken, token)
	})
}

func TestGetTokenData_TokenFile(t *testing.T) {
	testToken := "test-token-from-file"

	t.Run("reads token from file", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "token")
		require.NoError(t, os.WriteFile(tokenFile, []byte(testToken), 0600))

		token, err := GetTokenData("", tokenFile)
		require.NoError(t, err)
		assert.Equal(t, testToken, token)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		_, err := GetTokenData("", "/non/existent/path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token file")
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "k0s token create")
	})

	t.Run("returns error for empty file", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "empty-token")
		require.NoError(t, os.WriteFile(tokenFile, []byte{}, 0600))

		_, err := GetTokenData("", tokenFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token file")
		assert.Contains(t, err.Error(), "is empty")
		assert.Contains(t, err.Error(), "k0s token create")
	})
}

func TestGetTokenData_NoToken(t *testing.T) {
	t.Run("returns empty string when no token provided", func(t *testing.T) {
		t.Setenv(EnvVarToken, "")

		token, err := GetTokenData("", "")
		require.NoError(t, err)
		assert.Empty(t, token)
	})
}
