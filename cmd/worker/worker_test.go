// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/stretchr/testify/require"
)

func TestKubeconfigGetterFromJoinToken_NoSources(t *testing.T) {
	t.Setenv(internal.EnvVarToken, "")

	getter := kubeconfigGetterFromJoinToken("", "")
	require.Nil(t, getter)
}

func TestKubeconfigGetterFromJoinToken_TokenFileLazy(t *testing.T) {
	t.Setenv(internal.EnvVarToken, "")
	tokenFile := filepath.Join(t.TempDir(), "missing.token")

	getter := kubeconfigGetterFromJoinToken(tokenFile, "")
	require.NotNil(t, getter)

	_, err := getter()
	require.Error(t, err)
	require.Contains(t, err.Error(), tokenFile)
}

func TestKubeconfigGetterFromJoinToken_EnvVarDeferred(t *testing.T) {
	t.Setenv(internal.EnvVarToken, "not-base64")

	getter := kubeconfigGetterFromJoinToken("", "")
	require.NotNil(t, getter)

	_, err := getter()
	require.ErrorContains(t, err, "failed to decode join token")
}

func TestKubeconfigGetterFromJoinToken_InvalidArgDeferred(t *testing.T) {
	t.Setenv(internal.EnvVarToken, "")

	getter := kubeconfigGetterFromJoinToken("", "invalid")
	require.NotNil(t, getter)

	_, err := getter()
	require.ErrorContains(t, err, "failed to decode join token")
}
