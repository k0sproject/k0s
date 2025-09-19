//go:build linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package procfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// A process-specific subdirectory within the proc(5) file system, i.e., a
// /proc/<pid> directory. It exposes methods to parse the contents of the
// well-known files inside it.
type PIDDir struct{ fs.FS }

var _ fs.ReadFileFS = (*PIDDir)(nil)

// ReadFile implements [fs.ReadFileFS].
func (d *PIDDir) ReadFile(name string) (_ []byte, err error) {
	// The io/fs ReadFile implementation uses stat to optimize the read buffer
	// size by first determining the file size. This doesn't make sense, and is
	// even counter-productive for procfs files because they are usually
	// reported as having zero bytes, which is, of course, not what you get when
	// reading them. Hence PIDDir implements its own ReadFile method that skips
	// this step and allows the buffer to grow as needed.

	f, err := d.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	return io.ReadAll(f)
}

// Reads and parses /proc/<pid>/cmdline.
// https://man7.org/linux/man-pages/man5/proc_pid_cmdline.5.html
func (d *PIDDir) Cmdline() ([]string, error) {
	return d.readNulTerminatedStrings("cmdline")
}

// Reads and parses /proc/<pid>/environ.
// https://man7.org/linux/man-pages/man5/proc_pid_environ.5.html
func (d *PIDDir) Environ() ([]string, error) {
	return d.readNulTerminatedStrings("environ")
}

func (d *PIDDir) readNulTerminatedStrings(name string) (items []string, _ error) {
	raw, err := d.ReadFile(name)
	if err != nil {
		return nil, err
	}

	for len(raw) > 0 {
		current, rest, ok := bytes.Cut(raw, []byte{0})
		if !ok {
			return nil, fmt.Errorf("not properly terminated: %q", raw)
		}
		items = append(items, string(current))
		raw = rest
	}
	return items, nil
}
