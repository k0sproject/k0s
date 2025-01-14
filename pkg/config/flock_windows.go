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
	"golang.org/x/sys/windows"
	"os"
)

// tryLock attempts to acquire the lock. Returns true if successful, false otherwise.
func tryLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	handle := windows.Handle(file.Fd())
	overlapped := new(windows.Overlapped) // The OVERLAPPED structure, required for asynchronous I/O operations

	// Attempt to lock the file exclusively and fail immediately if it's already locked
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
	err = windows.LockFileEx(
		handle, // 1. HANDLE hFile: The handle to the file (must have GENERIC_READ or GENERIC_WRITE access)
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, // 2. DWORD dwFlags: Specifies the lock type and behavior
		0,          // 3. DWORD dwReserved: Reserved, must be zero
		1,          // 4. DWORD nNumberOfBytesToLockLow: Low-order part of the range of bytes to lock (1 byte in this case)
		0,          // 5. DWORD nNumberOfBytesToLockHigh: High-order part of the range of bytes to lock (0 for single-byte lock)
		overlapped, // 6. LPOVERLAPPED lpOverlapped: Pointer to an OVERLAPPED structure, required for this function
	)
	if err != nil {
		file.Close()
		if err == windows.ERROR_LOCK_VIOLATION {
			return nil, ErrK0sAlreadyRunning // Lock is already held by another process
		}
		return nil, err
	}

	return file, nil
}

// isLocked checks if the lock is currently held by another process.
func isLocked(path string) bool {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false
	}
	defer file.Close()

	handle := windows.Handle(file.Fd())
	overlapped := new(windows.Overlapped)

	// Try to acquire a shared lock without waiting
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
	err = windows.LockFileEx(
		handle,                            // 1. HANDLE hFile: The handle to the file (must have GENERIC_READ or GENERIC_WRITE access)
		windows.LOCKFILE_FAIL_IMMEDIATELY, // Try without waiting
		0,                                 // 3. DWORD dwReserved: Reserved, must be zero
		1,                                 // 4. DWORD nNumberOfBytesToLockLow: Low-order part of the range of bytes to lock (1 byte in this case)
		0,                                 // 5. DWORD nNumberOfBytesToLockHigh: High-order part of the range of bytes to lock (0 for single-byte lock)
		overlapped,                        // 6. LPOVERLAPPED lpOverlapped: Pointer to an OVERLAPPED structure, required for this function
	)
	return err != nil
}
