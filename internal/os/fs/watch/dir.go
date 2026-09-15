// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"errors"
)

// Indicates that a previously existing, actively watched directory has disappeared.
var ErrWatchedDirectoryGone = errors.New("watched directory is gone")

// Watches the directory specified by path and emits observed events to handler.
//
// The event stream is directory-relative:
//   - [*Established] is emitted once the watch has been established,
//   - [*Changed] is emitted for entries that appear, change, or disappear,
//   - [*Desynced] is emitted if the watch may have missed some changes.
//
// The function runs until ctx is done or watching fails. Note that this
// function only generates events for directory contents, never for the watched
// directory itself.
func Dir(ctx context.Context, path string, handler Handler) error {
	// Reasons for not generating events for the watched directory itself:
	//   1. This is not portable: Windows doesn't emit such events, and neither
	//      fsnotify nor we work around this.
	//   2. The vast majority of consumers supposedly aren't interested in such
	//      events and would need to filter them out themselves.

	return (&dirWatch{path, handler}).runFSNotify(ctx)
}

type dirWatch struct {
	path    string
	handler Handler
}
