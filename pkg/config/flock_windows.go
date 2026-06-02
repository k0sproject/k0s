//go:build windows

/*
Copyright 2025 k0s authors

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

package config

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(f *os.File, exclusive bool) (bool, error) {
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	var overlapped windows.Overlapped // The OVERLAPPED structure, required for asynchronous I/O operations
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}

	// Attempt to lock the file exclusively and fail immediately if it's already locked
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
