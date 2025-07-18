// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"golang.org/x/sys/unix"
)

func UnmountLazy(path string) error {
	return unix.Unmount(path, unix.MNT_DETACH)
}
