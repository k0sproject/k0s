//go:build unix

/*
Copyright 2020 k0s authors

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

package supervisor

import (
	"os"
	"syscall"
)

// DetachAttr creates the proper syscall attributes to run the managed processes
func DetachAttr(uid, gid int) *syscall.SysProcAttr {
	var creds *syscall.Credential

	if os.Geteuid() == 0 {
		creds = &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		}
	}

	return &syscall.SysProcAttr{
		Setpgid:    true,
		Pgid:       0,
		Credential: creds,
	}
}
