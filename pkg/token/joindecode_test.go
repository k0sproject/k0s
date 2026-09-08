// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"bytes"
	"testing"

	"github.com/k0sproject/k0s/pkg/token"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJoinToken_RoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := token.JoinEncode(bytes.NewReader([]byte("the-payload")))
	require.NoError(t, err)

	decoded, err := token.DecodeJoinToken(encoded)
	require.NoError(t, err)
	assert.Equal(t, []byte("the-payload"), decoded)
}

func TestDecodeJoinToken_InvalidBase64(t *testing.T) {
	t.Parallel()

	decoded, err := token.DecodeJoinToken("not-valid-base64!!!")
	assert.ErrorContains(t, err, "illegal base64 data")
	assert.Zero(t, decoded)
}

func TestDecodeJoinToken_InvalidGzip(t *testing.T) {
	t.Parallel()

	// Valid base64, but not gzip data underneath.
	decoded, err := token.DecodeJoinToken("bm90LWd6aXA=")
	assert.ErrorContains(t, err, "unexpected EOF")
	assert.Zero(t, decoded)
}

func TestGetTokenType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contexts map[string]*clientcmdapi.Context
		want     []string // map iteration order is unspecified, so any of these is acceptable
	}{
		{
			name: "zero contexts",
			want: []string{""},
		},
		{
			name: "one context",
			contexts: map[string]*clientcmdapi.Context{
				"the-context": {AuthInfo: "the-auth-info"},
			},
			want: []string{"the-auth-info"},
		},
		{
			name: "two contexts",
			contexts: map[string]*clientcmdapi.Context{
				"context-a": {AuthInfo: "auth-a"},
				"context-b": {AuthInfo: "auth-b"},
			},
			want: []string{"auth-a", "auth-b"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := &clientcmdapi.Config{Contexts: test.contexts}
			assert.Contains(t, test.want, token.GetTokenType(cfg))
		})
	}
}
