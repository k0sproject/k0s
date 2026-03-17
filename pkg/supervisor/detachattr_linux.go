//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// DetachAttr creates the proper syscall attributes to run the managed processes.
// The RequiredPrivileges are translated into Linux-specific ambient capabilities.
func DetachAttr(uid, gid int, privs RequiredPrivileges) *syscall.SysProcAttr {
	var creds *syscall.Credential

	if os.Geteuid() == 0 {
		creds = &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		}
	}

	var ambientCaps []uintptr
	if privs.BindsPrivilegedPorts {
		ambientCaps = []uintptr{unix.CAP_NET_BIND_SERVICE}
	}

	return &syscall.SysProcAttr{
		Setpgid:     true,
		Pgid:        0,
		Credential:  creds,
		AmbientCaps: ambientCaps,
	}
}
