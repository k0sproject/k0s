// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"io"
)

// A handle to a running process. May be used to inspect the process properties
// and terminate it.
type procHandle interface {
	io.Closer

	// Checks whether the process has terminated.
	hasTerminated() (bool, error)

	// Reads and returns the process's command line.
	cmdline() ([]string, error)

	// Reads and returns the process's environment.
	environ() ([]string, error)

	// Requests graceful process termination.
	requestGracefulTermination() error

	// Kills the process.
	kill() error
}
