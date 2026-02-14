//go:build linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package procfs

import (
	"bytes"
	"fmt"
)

type PIDState byte

// Known values of the process state values as used in the third field of /proc/<pid>/stat.
const (
	PIDStateRunning     PIDState = 'R'
	PIDStateSleeping    PIDState = 'S' // in an interruptible wait
	PIDStateWaiting     PIDState = 'D' // in uninterruptible disk sleep
	PIDStateZombie      PIDState = 'Z'
	PIDStateStopped     PIDState = 'T' // (on a signal) or (before Linux 2.6.33) trace stopped
	PIDStateTracingStop PIDState = 't' // (Linux 2.6.33 onward)
	PIDStatePaging      PIDState = 'W' // (only before Linux 2.6.0)
	PIDStateDead        PIDState = 'X' // (from Linux 2.6.0 onward)
	PIDStateDeadX       PIDState = 'x' // (Linux 2.6.33 to 3.13 only)
	PIDStateWakekill    PIDState = 'K' // (Linux 2.6.33 to 3.13 only)
	PIDStateWaking      PIDState = 'W' // (Linux 2.6.33 to 3.13 only)
	PIDStateParked      PIDState = 'P' // (Linux 3.9 to 3.13 only)
	PIDStateIdle        PIDState = 'I' // (Linux 4.14 onward)
)

// Reads the state field from /proc/<pid>/stat.
// https://man7.org/linux/man-pages/man5/proc_pid_stat.5.html
func (d *PIDDir) State() (PIDState, error) {
	raw, err := d.ReadFile("stat")
	if err != nil {
		return 0, err
	}

	// Skip over the pid and comm fields: The last parenthesis marks the end of
	// the comm field, all other fields won't contain parentheses. The end of
	// comm needs to be at the fourth byte the earliest.
	if idx := bytes.LastIndexByte(raw, ')'); idx < 0 {
		return 0, fmt.Errorf("no closing parenthesis: %q", raw)
	} else {
		raw = raw[idx+1:]
	}

	if len(raw) < 3 || raw[0] != ' ' || raw[2] != ' ' {
		return 0, fmt.Errorf("failed to locate state field: %q", raw)
	}

	return PIDState(raw[1]), nil
}
