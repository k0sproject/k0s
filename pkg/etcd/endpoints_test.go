// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
)

func TestSocketURL(t *testing.T) {
	assert.Equal(t, "unixs:///run/k0s/etcd/localhost:2379", SocketURL("/run/k0s/etcd/localhost:2379"),
		"absolute paths should become the URL's path")
	// Non-rooted paths get a leading slash, so that the path's first segment
	// is never mistaken for a URL host (e.g. Windows drive letters).
	assert.Equal(t, "unixs:///C:/k0s/run/etcd/localhost:2379", SocketURL("C:/k0s/run/etcd/localhost:2379"),
		"non-rooted paths should get a leading slash")
}

func TestEndpoints(t *testing.T) {
	const socketPath = "/run/k0s/etcd/localhost:2379"
	const socketURL = "unixs:///run/k0s/etcd/localhost:2379"

	t.Run("uses the socket URL for internal etcd", func(t *testing.T) {
		assert.Equal(t, []string{socketURL}, Endpoints(v1beta1.DefaultEtcdConfig(), socketPath),
			"internal etcd should be reached via its socket")
	})

	t.Run("uses the socket URL without config", func(t *testing.T) {
		assert.Equal(t, []string{socketURL}, Endpoints(nil, socketPath),
			"a nil config should be treated as internal etcd")
	})

	t.Run("uses the external cluster's endpoints", func(t *testing.T) {
		etcdConf := &v1beta1.EtcdConfig{
			ExternalCluster: &v1beta1.ExternalCluster{
				Endpoints: []string{"https://192.168.10.10:2379", "https://192.168.10.11:2379"},
			},
		}
		assert.Equal(t, []string{"https://192.168.10.10:2379", "https://192.168.10.11:2379"}, Endpoints(etcdConf, socketPath),
			"external cluster endpoints should be returned as configured")
	})
}
