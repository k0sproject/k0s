//go:build windows

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"os"

	"golang.org/x/sys/windows"
)

func lock(f *os.File, exclusive, wait bool) (bool, error) {
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if wait {
		flags = 0
	}
	var overlapped windows.Overlapped // The OVERLAPPED structure, required for asynchronous I/O operations
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}

	// Attempt to lock the file, and unless asked to wait, fail immediately if it's already locked
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
	if err := windows.LockFileEx(
		/* HANDLE hFile                   */ windows.Handle(f.Fd()), // The handle to the file (must have GENERIC_READ or GENERIC_WRITE access)
		/* DWORD dwFlags                  */ flags, // Specifies the lock type and behavior
		/* DWORD dwReserved               */ 0, // Reserved, must be zero
		/* DWORD nNumberOfBytesToLockLow  */ 1, // Low-order part of the range of bytes to lock (1 byte in this case)
		/* DWORD nNumberOfBytesToLockHigh */ 0, // High-order part of the range of bytes to lock (0 for single-byte lock)
		/* LPOVERLAPPED lpOverlapped      */ &overlapped, // Pointer to an OVERLAPPED structure, required for this function
	); err == nil {
		return true, nil
	} else if err == windows.ERROR_LOCK_VIOLATION { //nolint:errorlint // safe for syscalls
		// Lock is already held by another process
		return false, nil
	} else {
		return false, os.NewSyscallError("LockFileEx", err)
	}
}
