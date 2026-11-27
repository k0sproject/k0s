// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"errors"
	"io/fs"
)

var _ = errors.Is // for godoc links

// A watch event.
//
// Callers typically inspect concrete event values such as [*Established],
// [*Changed], and [*Desynced] using a type switch.
type Event interface {
	watchEvent() // Marker method to be implemented, to distinguish this from [any].
}

// Reports that a watch has been established, i.e. it is in effect and any
// subsequent changes to the watched path will be noticed.
type Established struct {
	// The path being watched.
	Path string
}

func (*Established) watchEvent() {}

// Reports that a path has been created, changed, or has disappeared.
type Changed struct {
	// The base name of the changed path, relative to the watched path.
	Name string

	// May be used to inspect the changed path, avoiding the need for the caller
	// to perform path manipulations. Never nil. [PathGone] if the watch
	// observed the path disappearing.
	PathInfo
}

func (*Changed) watchEvent() {}

// Provides information about a path on demand.
type PathInfo interface {
	// Returns information about the path itself, without following symlinks.
	// Depending on the backing implementation, the results may or may not be
	// cached, enabling implementations to avoid an extra syscall when the
	// metadata is already known.
	Stat() (fs.FileInfo, error)
}

// Adapts a function to the [PathInfo] interface.
type PathInfoFunc func() (fs.FileInfo, error)

func (f PathInfoFunc) Stat() (fs.FileInfo, error) { return f() }

// The [PathInfo] of paths that the watch observed disappearing. It's a
// [PathInfo] and an error at the same time. Its Stat method returns PathGone
// itself, which satisfies [errors.Is] for [fs.ErrNotExist]. Consumers that need
// to know if the watch itself observed a path disappearing may compare against
// this. Otherwise, it suffices to check Stat's error against [fs.ErrNotExist].
var PathGone interface {
	PathInfo
	error
} = gone{}

type gone struct{}

func (gone) Stat() (fs.FileInfo, error) { return nil, PathGone }
func (gone) Error() string              { return "watched path is gone" }
func (gone) Is(other error) bool        { return other == fs.ErrNotExist || other == PathGone }

// Reports that a watch may have missed some changes in the watched path.
// Consumers that need to retain consistency with the file system must resync
// with it.
type Desynced struct {
	// The path being watched.
	Path string
}

func (*Desynced) watchEvent() {}

// Consumes [Event] values.
type Handler interface {
	Handle(e Event)
}

// Adapts a function to the [Handler] interface.
type HandlerFunc func(e Event)

func (f HandlerFunc) Handle(e Event) { f(e) }
