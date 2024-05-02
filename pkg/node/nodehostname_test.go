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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	nodeutil "k8s.io/component-helpers/node/util"
)

func TestGetNodename(t *testing.T) {

	startFakeMetadataServer(":8080")
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
		name, err := getNodeNameWindows("", "http://localhost:8080")
		nodename, err2 := nodeutil.GetHostname("")
		require.Nil(t, err)
		require.Nil(t, err2)
		require.Equal(t, nodename, name)
	})

	t.Run("windows_metadata_service_is_available", func(t *testing.T) {
		name, err := getNodeNameWindows("", "http://localhost:8080/latest/meta-data/local-hostname")
		nodename, err2 := nodeutil.GetHostname("")
		require.Nil(t, err)
		require.Nil(t, err2)
		require.NotEqual(t, nodename, name)
	})
}

func startFakeMetadataServer(listenOn string) {
	go func() {
		http.HandleFunc("/latest/meta-data/local-hostname", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("some-hostname-from-metadata"))
			w.WriteHeader(http.StatusOK)
		})
		_ = http.ListenAndServe(listenOn, nil)
	}()
}
