// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type dirEntry struct {
	name string
	info pathInfo
}

type pathInfo struct {
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (i *pathInfo) Equal(other *pathInfo) bool {
	return i.size == other.size &&
		i.mode == other.mode &&
		i.modTime.Equal(other.modTime)
}

func (d *dirWatch) runPolling(log logrus.FieldLogger, done <-chan struct{}, nextPollInterval func() time.Duration) error {
	var currentEntries []dirEntry
	if dirEntries, err := os.ReadDir(d.path); err != nil {
		return err
	} else if currentEntries, err = diffDirEntries(dirEntries, nil, func(Event) { /* no-op */ }); err != nil {
		return fmt.Errorf("initial poll failed: %w", err)
	}

	d.handler.Handle(&Established{Path: d.path})

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-done:
			return nil
		case <-timer.C:
			log.Debug("Polling for changes")
			if dirEntries, err := os.ReadDir(d.path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("%w: %w", ErrWatchedDirectoryGone, err)
				}
				return err
			} else if currentEntries, err = diffDirEntries(dirEntries, currentEntries, d.handler.Handle); err != nil {
				return fmt.Errorf("failed to poll: %w", err)
			}
			timer.Reset(nextPollInterval())
		}
	}
}

// Diffs the given entries with a previously collected snapshot of directory
// entries and emits the resulting events to handler. Returns an up-to-date
// snapshot for subsequent diffs.
//
// Both entries and prev must be ordered by entry name in ascending lexical
// order. [os.ReadDir] satisfies this requirement for entries, and prev is
// expected to be the previous snapshot returned by diffDirEntries, which
// preserves that ordering.
func diffDirEntries(entries []os.DirEntry, prev []dirEntry, handle func(Event)) ([]dirEntry, error) {
	current := make([]dirEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}

		current = append(current, dirEntry{
			entry.Name(),
			pathInfo{
				info.Size(),
				info.Mode(),
				info.ModTime(),
			},
		})

		var unchanged bool
		for i, prevLen := 0, len(prev); ; i++ {
			if i == prevLen {
				prev = nil
			} else if cmp := strings.Compare(prev[i].name, entry.Name()); cmp < 0 {
				handle(&Gone{prev[i].name})
				continue
			} else if cmp == 0 {
				unchanged = prev[i].info.Equal(&current[len(current)-1].info)
				prev = prev[i+1:]
			} else {
				prev = prev[i:]
			}
			break
		}

		if !unchanged {
			handle(&Touched{entry.Name(), func() (fs.FileInfo, error) {
				return info, nil
			}})
		}
	}

	for i := range prev {
		handle(&Gone{prev[i].name})
	}

	return current, nil
}
