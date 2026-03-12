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

// Decides whether an [Event] is relevant to a consumer or not.
type Predicate func(Event) bool

// Returns a predicate that rejects [*Established] events and accepts all other
// events.
//
// This is useful when a caller only wants to react to actual file system events
// and is not interested in the time at which the watch becomes effective.
func RejectEstablished() Predicate {
	return func(e Event) bool { _, ok := e.(*Established); return !ok }
}

// Returns a predicate that rejects [*Touched] and [*Gone] events for names for
// which deny returns true.
//
// All other events are always accepted.
func RejectNames(deny func(string) bool) Predicate {
	return func(e Event) bool {
		switch e := e.(type) {
		case *Touched:
			return !deny(e.Name)
		case *Gone:
			return !deny(e.Name)
		default:
			return true
		}
	}
}

// Consumes [Event] values.
type Handler interface {
	Handle(e Event)
}

// Adapts a function to the [Handler] interface.
type HandlerFunc func(e Event)

func (f HandlerFunc) Handle(e Event) { f(e) }
