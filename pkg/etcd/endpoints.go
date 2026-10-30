// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// SocketURL returns the TLS-enabled (unixs) URL for the internal etcd's
// client unix socket at the given file system path.
//
// Note that the etcd client derives the TLS server name from the socket's
// base name, stripping any port suffix, so the base name has to match a SAN
// of the etcd server certificate (see [constant.EtcdSocket]).
func SocketURL(socketPath string) string {
	socketPath = filepath.ToSlash(socketPath)
	// Ensure the URL is always in the canonical absolute form (unixs:///path),
	// so that the path is never mistaken for a host (e.g. Windows drive letters).
	if !strings.HasPrefix(socketPath, "/") {
		socketPath = "/" + socketPath
	}
	return (&url.URL{Scheme: "unixs", Path: socketPath}).String()
}

// Endpoints returns the external cluster endpoints, if configured, or the
// internal etcd's client unix socket URL for the given socket path.
func Endpoints(etcdConf *v1beta1.EtcdConfig, socketPath string) []string {
	if etcdConf != nil && etcdConf.IsExternalClusterUsed() {
		return etcdConf.ExternalCluster.Endpoints
	}
	return []string{SocketURL(socketPath)}
}
