// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/cmd/internal"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"

	"github.com/stretchr/testify/assert"
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

func TestShouldFallBackToCachedProfile(t *testing.T) {
	t.Parallel()

	unreachable := &workerconfig.APIUnreachableError{Err: errors.New("connection refused")}

	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{"unreachable_api_server", unreachable, true},
		{"wrapped_unreachable_api_server", fmt.Errorf("failed to load the worker profile: %w", unreachable), true},
		{"unrelated_failure", assert.AnError, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, shouldFallBackToCachedProfile(test.err),
				"Wrong decision for %v", test.err)
		})
	}
}
