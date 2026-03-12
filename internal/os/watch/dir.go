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
//   - [*Touched] is emitted for entries that appear or change,
//   - [*Gone] is emitted for entries that disappear.
//
// The function runs until ctx is done or watching fails.
func Dir(ctx context.Context, path string, handler Handler) error {
	return (&dirWatch{path, handler}).runFSNotify(ctx)
}

type dirWatch struct {
	path    string
	handler Handler
}
