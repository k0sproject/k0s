// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import "syscall"

// DetachAttr creates the proper syscall attributes to run the managed processes.
// Puts processes into their own process group, so that Ctrl+Break events will only
// affect the spawned processes, but not k0s itself.
// The RequiredPrivileges parameter is ignored on Windows as it's a Linux-specific feature.
func DetachAttr(uid, gid int, privs RequiredPrivileges) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
