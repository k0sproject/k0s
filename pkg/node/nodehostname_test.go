package node

import (
	"net/http"
	"testing"

	"github.com/davecgh/go-spew/spew"
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

	t.Run("windows_metadata_service_is_broken", func(t *testing.T) {
		name, err := getNodeNameWindows("", "http://localhost:8080/not-found")
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
		spew.Dump(name, nodename, err, err2)
		require.NotEqual(t, nodename, name)
	})
}

func startFakeMetadataServer(listenOn string) {
	go func() {
		http.HandleFunc("/latest/meta-data/local-hostname", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("some-hostname-from-metadata"))
			w.WriteHeader(http.StatusOK)
		})
		http.HandleFunc("/not-found", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		http.ListenAndServe(listenOn, nil)
	}()
}
