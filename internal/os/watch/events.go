// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"io/fs"
)

// A watch event.
//
// Callers typically inspect concrete event values such as [*Established],
// [*Touched], and [*Gone] using a type switch.
type Event interface {
	WatchEvent() // Marker method to be implemented, to distinguish this from [any].
}

// Reports that a watch has been established, i.e. it is in effect and any
// subsequent changes to the watched path will be noticed.
type Established struct {
	// The path being watched.
	Path string
}

func (*Established) WatchEvent() {}

// Reports that a path has been created or changed in any way other than
// disappearing.
type Touched struct {
	// The base name of the touched path, relative to the watched path.
	Name string

	// May be used to stat the touched path, avoiding the need for the caller to
	// perform path manipulations. Depending on the backing implementation, the
	// results may or may not be cached, enabling implementations to avoid an
	// extra stat when the metadata is already known.
	Info func() (fs.FileInfo, error)
}

func (*Touched) WatchEvent() {}

// Reports that a path that previously existed has disappeared.
type Gone struct {
	// The base name of the path that disappeared, relative to the watched path.
	Name string
}

func (*Gone) WatchEvent() {}

// Consumes [Event] values.
type Handler interface {
	Handle(e Event)
}

// Adapts a function to the [Handler] interface.
type HandlerFunc func(e Event)

func (f HandlerFunc) Handle(e Event) { f(e) }
