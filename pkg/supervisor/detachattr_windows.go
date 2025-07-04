// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import "syscall"

// DetachAttr creates the proper syscall attributes to run the managed processes
// on windows it doesn't use any arguments but just to keep signature similar
func DetachAttr(int, int) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
