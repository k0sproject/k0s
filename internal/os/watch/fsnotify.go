// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

func (d *dirWatch) runFSNotify(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer func() { err = errors.Join(err, watcher.Close()) }()

	if err := watcher.Add(d.path); err != nil {
		return fmt.Errorf("failed to watch: %w", err)
	}

	d.handler.Handle(&Established{Path: d.path})

	for {
		select {
		case event := <-watcher.Events:
			name, err := filepath.Rel(d.path, event.Name)
			if err != nil {
				return fmt.Errorf("while normalizing event name: %w", err)
			}
			switch {
			case event.Has(fsnotify.Remove):
				if event.Name == d.path {
					return fmt.Errorf("%w: removed", ErrWatchedDirectoryGone)
				}
				d.handler.Handle(&Gone{Name: name})

			case event.Has(fsnotify.Rename):
				if event.Name == d.path {
					return fmt.Errorf("%w: renamed", ErrWatchedDirectoryGone)
				}
				d.handler.Handle(&Gone{Name: name})

			case event.Has(fsnotify.Create), event.Has(fsnotify.Write), event.Has(fsnotify.Chmod):
				d.handler.Handle(&Touched{
					Name: name,
					Info: sync.OnceValues(func() (fs.FileInfo, error) {
						return os.Stat(event.Name)
					}),
				})

			default:
				return fmt.Errorf("unknown event: %v", event)
			}

		case err := <-watcher.Errors:
			return fmt.Errorf("while watching: %w", err)
		case <-ctx.Done():
			return nil
		}
	}
}
