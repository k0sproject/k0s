// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package constant

const (
	// DataDirDefault is the default directory containing k0s state.
	DataDirDefault = "C:\\var\\lib\\k0s"

	KineSocket = "kine\\kine.sock:2379"
	// EtcdSocket is the path of etcd's client unix socket, relative to the
	// run directory. See the unix constant for why it's named like this.
	EtcdSocket              = "etcd\\localhost:2379"
	K0sConfigPathDefault    = "C:\\etc\\k0s\\k0s.yaml"
	StatusSocketPathDefault = `\\.\pipe\k0s-status`

	ExecutableSuffix = ".exe"
)
