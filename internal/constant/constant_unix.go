//go:build unix

// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package constant

const (
	// DataDirDefault is the default directory containing k0s state.
	DataDirDefault = "/var/lib/k0s"

	KineSocket              = "kine/kine.sock:2379"
	K0sConfigPathDefault    = "/etc/k0s/k0s.yaml"
	StatusSocketPathDefault = "/run/k0s/status.sock"

	ExecutableSuffix = ""
)
