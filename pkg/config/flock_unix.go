//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"golang.org/x/sys/unix"
	"os"
)

// tryLock attempts to acquire the lock. Returns *os.File if successful, nil otherwise.
func tryLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if err == unix.EWOULDBLOCK {
			return nil, ErrK0sAlreadyRunning // Lock is already held by another process
		}
		return nil, err
	}
	return file, nil
}

// isLocked checks if the lock is currently held by another process.
func isLocked(path string) bool {
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return false
	}
	defer file.Close()

	// Attempt a non-blocking shared lock to test the lock state
	if err := unix.Flock(int(file.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		return err == unix.EWOULDBLOCK
	}

	return false
}
