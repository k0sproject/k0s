/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package node

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/k0sproject/k0s/pkg/k0scontext"

	apitypes "k8s.io/apimachinery/pkg/types"
	nodeutil "k8s.io/component-helpers/node/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetNodeNameWindows(t *testing.T) {
	kubeHostname, err := nodeutil.GetHostname("")
	require.NoError(t, err)
	baseURL := startFakeMetadataServer(t)

	t.Run("no_metadata_service_available", func(t *testing.T) {
		ctx := k0scontext.WithValue(context.TODO(), nodenameURL(baseURL))
		name, err := getNodeName(ctx, "")
		if assert.NoError(t, err) {
			assert.Equal(t, apitypes.NodeName(kubeHostname), name)
		}
	})

	t.Run("metadata_service_is_available", func(t *testing.T) {
		ctx := k0scontext.WithValue(context.TODO(), nodenameURL(baseURL+"/latest/meta-data/local-hostname"))
		name, err := getNodeName(ctx, "")
		if assert.NoError(t, err) {
			assert.Equal(t, apitypes.NodeName("some-hostname from aws_metadata"), name)
		}
	})
}

func startFakeMetadataServer(t *testing.T) string {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/meta-data/local-hostname", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("Some-hostname from AWS_metadata\n"))
		assert.NoError(t, err)
	})
	server := &http.Server{Addr: "localhost:0", Handler: mux}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		require.NoError(t, err)
	}

	serverError := make(chan error)
	go func() {
		defer close(serverError)
		serverError <- server.Serve(listener)
	}()

	t.Cleanup(func() {
		err := server.Shutdown(context.Background())
		if !assert.NoError(t, err, "Couldn't shutdown HTTP server") {
			return
		}

		assert.ErrorIs(t, <-serverError, http.ErrServerClosed, "HTTP server terminated unexpectedly")
	})

	return (&url.URL{Scheme: "http", Host: listener.Addr().String()}).String()
}
