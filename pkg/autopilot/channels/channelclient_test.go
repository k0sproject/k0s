// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package channels

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k0sproject/k0s/internal/secret"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelClient_GetLatest(t *testing.T) {
	var authorization []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Values("Authorization")
		_, _ = w.Write([]byte("version: v99.99.99+k0s.0\n"))
	}))
	t.Cleanup(server.Close)

	t.Run("sends the token as bearer token", func(t *testing.T) {
		underTest, err := NewChannelClient(server.URL, "stable", secret.FromString("c0nf1d3n714l"))
		require.NoError(t, err)

		latest, err := underTest.GetLatest(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "v99.99.99+k0s.0", latest.Version)
		assert.Equal(t, []string{"Bearer c0nf1d3n714l"}, authorization, "Request should be authenticated with the token")
	})

	t.Run("sends no authorization without a token", func(t *testing.T) {
		underTest, err := NewChannelClient(server.URL, "stable", secret.String{})
		require.NoError(t, err)

		_, err = underTest.GetLatest(t.Context(), nil)
		require.NoError(t, err)
		assert.Empty(t, authorization, "Request should be unauthenticated without a token")
	})
}

func TestNewChannelClientChannelURL(t *testing.T) {
	type args struct {
		server  string
		channel string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "full URL",
			args: args{
				server:  "https://example.com",
				channel: "foo",
			},
			want: "https://example.com/foo/index.yaml",
		},
		{
			name: "partial URL",
			args: args{
				server:  "example.com",
				channel: "foo",
			},
			want: "https://example.com/foo/index.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewChannelClient(tt.args.server, tt.args.channel, secret.String{})
			if (err != nil) != tt.wantErr {
				t.Errorf("NewChannelClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.channelURL != tt.want {
				t.Errorf("NewChannelClient() = %v, want %v", got.channelURL, tt.want)
			}
		})
	}
}
