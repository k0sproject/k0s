// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k0sproject/k0s/internal/secret"
	"github.com/k0sproject/k0s/pkg/autopilot/updater"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetUpdate(t *testing.T) {
	var authorization []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Values("Authorization")
		_, _ = w.Write([]byte("version: v99.99.99+k0s.0\n"))
	}))
	t.Cleanup(server.Close)

	t.Run("sends the token as bearer token", func(t *testing.T) {
		underTest, err := updater.NewClient(server.URL, secret.FromString("c0nf1d3n714l"))
		require.NoError(t, err)

		update, err := underTest.GetUpdate("stable", "some-cluster", "", "v1.0.0+k0s.0")
		require.NoError(t, err)
		assert.Equal(t, updater.Version("v99.99.99+k0s.0"), update.Version)
		assert.Equal(t, []string{"Bearer c0nf1d3n714l"}, authorization, "Request should be authenticated with the token")
	})

	t.Run("sends no authorization without a token", func(t *testing.T) {
		underTest, err := updater.NewClient(server.URL, secret.String{})
		require.NoError(t, err)

		_, err = underTest.GetUpdate("stable", "some-cluster", "", "v1.0.0+k0s.0")
		require.NoError(t, err)
		assert.Empty(t, authorization, "Request should be unauthenticated without a token")
	})
}
