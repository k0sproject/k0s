/*
Copyright 2023 k0s authors

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nodeutil "k8s.io/component-helpers/node/util"
)

func TestGetNodename(t *testing.T) {

	baseURL := startFakeMetadataServer(t)
	t.Run("should_always_return_override_if_given", func(t *testing.T) {
		name, err := GetNodename("override")
		require.Equal(t, "override", name)
		require.Nil(t, err)
	})

	t.Run("should_call_kubernetes_hostname_helper_on_linux", func(t *testing.T) {
		name, err := GetNodename("")
		name2, err2 := nodeutil.GetHostname("")
		require.Equal(t, name, name2)
		require.Nil(t, err)
		require.Nil(t, err2)
	})

	t.Run("windows_no_metadata_service_available", func(t *testing.T) {
		name, err := getNodeNameWindows("", baseURL)
		nodename, err2 := nodeutil.GetHostname("")
		require.Nil(t, err)
		require.Nil(t, err2)
		require.Equal(t, nodename, name)
	})

	t.Run("windows_metadata_service_is_available", func(t *testing.T) {
		name, err := getNodeNameWindows("", baseURL+"/latest/meta-data/local-hostname")
		nodename, err2 := nodeutil.GetHostname("")
		require.Nil(t, err)
		require.Nil(t, err2)
		require.NotEqual(t, nodename, name)
	})
}

func startFakeMetadataServer(t *testing.T) string {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/meta-data/local-hostname", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("some-hostname-from-metadata"))
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
