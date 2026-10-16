// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"errors"
	"fmt"
	"os"
)

// TryLock locks the file without waiting and reports whether it got the lock. Closing the file releases the lock.
func TryLock(f *os.File, exclusive bool) (bool, error) {
	return lock(f, exclusive, false)
}

// LockPath opens the file, creating it if needed, and waits until it gets the lock. Closing the file releases the lock.
func LockPath(path string, exclusive bool) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if _, err = lock(f, exclusive, true); err != nil {
		return nil, errors.Join(fmt.Errorf("failed to lock %s: %w", path, err), f.Close())
	}
	return f, nil
}
