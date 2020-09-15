// +build !windows

package supervisor

import "syscall"

// DetachAttr creates the proper syscall attributes to run the managed processes
func DetachAttr(uid, gid int) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}
}
