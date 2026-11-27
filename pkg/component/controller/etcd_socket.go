// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// The mode of the etcd client unix socket, so that it's connectable by other
// local users, most notably kube-apiserver. Not an access control: etcd serves
// the socket with TLS client certificate authentication.
const etcdSocketMode = 0666

// watchSocketMode keeps the mode of the unix socket at socketPath at
// [etcdSocketMode]. It watches the socket's directory and adjusts the socket's
// mode whenever it gets (re-)created or its mode gets changed, e.g. when etcd
// binds the socket on each (re-)start, honoring its umask. The watch is
// established before this function returns, so that no socket creation that
// happens afterwards will go unnoticed. The returned function stops the watch
// and waits for it to terminate.
func watchSocketMode(log logrus.FieldLogger, socketPath string) (stop func(), _ error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(filepath.Dir(socketPath)); err != nil {
		return nil, errors.Join(err, watcher.Close())
	}

	ensureMode := func() {
		switch changed, err := ensureUnixSocketMode(socketPath, etcdSocketMode); {
		case errors.Is(err, os.ErrNotExist): // the socket hasn't been bound (yet)
		case err != nil:
			log.WithError(err).Error("Failed to adjust socket mode, local etcd clients such as kube-apiserver may be unable to connect")
		case changed:
			log.WithField("path", socketPath).Debugf("Adjusted socket mode to %o", etcdSocketMode)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if err := watcher.Close(); err != nil {
				log.WithError(err).Error("Failed to close socket watcher")
			}
		}()

		ensureMode() // The socket might already be there.

		socketPath := filepath.Clean(socketPath)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) == socketPath && event.Has(fsnotify.Create|fsnotify.Chmod) {
					ensureMode()
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Events may have been lost, so re-check the socket.
				log.WithError(err).Warn("Error while watching socket")
				ensureMode()

			case <-ctx.Done():
				return
			}
		}
	}()

	return func() { cancel(); <-done }, nil
}
