//go:build !linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"os"

	"github.com/fsnotify/fsnotify"
)

func newFSNotifyWatcher() (*fsnotifyWatcher, error, bool) {
	watcher, err := fsnotify.NewWatcher()
	return (*fsnotifyWatcher)(watcher), err, false
}

func (w *fsnotifyWatcher) add(path string) (error, bool) {
	err := (*fsnotify.Watcher)(w).Add(path)
	//nolint:errorlint // Only wrap otherwise unwrapped errors.
	if syscallErr, ok := err.(*os.SyscallError); ok {
		err = &os.PathError{
			Op:   syscallErr.Syscall,
			Path: path,
			Err:  syscallErr.Err,
		}
	}
	return err, false
}
