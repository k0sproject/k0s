//go:build unix

// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package constant

const (
	// DataDirDefault is the default directory containing k0s state.
	DataDirDefault = "/var/lib/k0s"

	KineSocket = "kine/kine.sock:2379"
	// EtcdSocket is the path of etcd's client unix socket, relative to the
	// run directory. Note that despite its looks, this is NOT a TCP address:
	// it's the file name of the unix socket. The etcd client derives the TLS
	// server name from a unix socket's base name, stripping any port suffix,
	// so this particular name ensures that TLS connections are verified
	// against the localhost SAN of the etcd server certificate.
	EtcdSocket              = "etcd/localhost:2379"
	K0sConfigPathDefault    = "/etc/k0s/k0s.yaml"
	StatusSocketPathDefault = "/run/k0s/status.sock"

	ExecutableSuffix = ""
)
