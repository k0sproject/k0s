// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/token"

	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinClient_GetCA(t *testing.T) {
	t.Parallel()

	joinURL, certData := startFakeJoinServer(t, func(res http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/some/sub/path/v1beta1/ca", req.RequestURI)
		assert.Equal(t, []string{"Bearer the-id.the-secret"}, req.Header["Authorization"])
		_, err := res.Write([]byte("{}"))
		assert.NoError(t, err)
	})

	joinURL.Path = "/some/sub/path"
	kubeconfig, err := token.GenerateKubeconfig(joinURL.String(), certData, token.ControllerTokenAuthName, &bootstraptokenv1.BootstrapTokenString{ID: "the-id", Secret: "the-secret"})
	require.NoError(t, err)
	tok, err := token.JoinEncode(bytes.NewReader(kubeconfig))
	require.NoError(t, err)

	underTest, err := token.JoinClientFromToken(tok)
	require.NoError(t, err)

	response, err := underTest.GetCA(t.Context())
	assert.NoError(t, err)
	assert.Zero(t, response)
}

func TestJoinClient_JoinEtcd(t *testing.T) {
	t.Parallel()

	joinURL, certData := startFakeJoinServer(t, func(res http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/some/sub/path/v1beta1/etcd/members", req.RequestURI)
		assert.Equal(t, []string{"Bearer the-id.the-secret"}, req.Header["Authorization"])

		if body, err := io.ReadAll(req.Body); assert.NoError(t, err) {
			var data map[string]string
			if assert.NoError(t, json.Unmarshal(body, &data)) {
				assert.Equal(t, map[string]string{
					"node":        "the-node",
					"peerAddress": "the-peer-address",
				}, data)
			}
		}

		_, err := res.Write([]byte("{}"))
		assert.NoError(t, err)
	})

	joinURL.Path = "/some/sub/path"
	kubeconfig, err := token.GenerateKubeconfig(joinURL.String(), certData, token.ControllerTokenAuthName, &bootstraptokenv1.BootstrapTokenString{ID: "the-id", Secret: "the-secret"})
	require.NoError(t, err)
	tok, err := token.JoinEncode(bytes.NewReader(kubeconfig))
	require.NoError(t, err)

	underTest, err := token.JoinClientFromToken(tok)
	require.NoError(t, err)

	response, err := underTest.JoinEtcd(t.Context(), k0sv1beta1.EtcdRequest{
		Node:        "the-node",
		PeerAddress: "the-peer-address",
	})
	assert.NoError(t, err)
	assert.Zero(t, response)
}

func TestJoinClient_Cancellation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		funcUnderTest func(context.Context, *token.JoinClient) error
	}{
		{"GetCA", func(ctx context.Context, c *token.JoinClient) error {
			_, err := c.GetCA(ctx)
			return err
		}},
		{"JoinEtcd", func(ctx context.Context, c *token.JoinClient) error {
			_, err := c.JoinEtcd(ctx, k0sv1beta1.EtcdRequest{})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clientContext, cancelClientContext := context.WithCancelCause(t.Context())
			joinURL, certData := startFakeJoinServer(t, func(_ http.ResponseWriter, req *http.Request) {
				cancelClientContext(assert.AnError) // cancel the client's context
				<-req.Context().Done()              // block forever
			})

			kubeconfig, err := token.GenerateKubeconfig(joinURL.String(), certData, token.ControllerTokenAuthName, &bootstraptokenv1.BootstrapTokenString{})
			require.NoError(t, err)
			tok, err := token.JoinEncode(bytes.NewReader(kubeconfig))
			require.NoError(t, err)

			underTest, err := token.JoinClientFromToken(tok)
			require.NoError(t, err)

			err = test.funcUnderTest(clientContext, underTest)
			assert.ErrorIs(t, err, context.Canceled, "Expected the call to be canceled")
			assert.Same(t, context.Cause(clientContext), assert.AnError, "Didn't receive an HTTP request")
		})
	}
}

func startFakeJoinServer(t *testing.T, handler func(http.ResponseWriter, *http.Request)) (*url.URL, []byte) {
	requestCtx, cancelRequests := context.WithCancel(t.Context())
	var ok bool

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		require.NoError(t, err)
	}
	defer func() {
		if !ok {
			assert.NoError(t, listener.Close())
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	certData, _, keyData, err := initca.New(&csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
		CN:         fmt.Sprintf("localhost:%d", addr.Port),
		Hosts:      []string{addr.IP.String()},
	})
	if !assert.NoError(t, err) {
		assert.NoError(t, listener.Close())
		t.FailNow()
	}
	cert, err := tls.X509KeyPair(certData, keyData)
	require.NoError(t, err)

	server := &http.Server{
		Addr: addr.String(),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		Handler:     http.HandlerFunc(handler),
		BaseContext: func(net.Listener) context.Context { return requestCtx },
	}
	serverError := make(chan error)
	ok = true
	go func() { defer close(serverError); serverError <- server.ServeTLS(listener, "", "") }()
	t.Cleanup(func() {
		cancelRequests()
		// We cannot use t.Context because this is during t.Cleanup because it's canceled
		if !assert.NoError(t, server.Shutdown(testutil.ContextBackground()), "Couldn't shutdown HTTP server") {
			return
		}
		assert.ErrorIs(t, <-serverError, http.ErrServerClosed, "HTTP server terminated unexpectedly")
	})

	return &url.URL{Scheme: "https", Host: server.Addr}, certData
}
