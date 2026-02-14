//go:build linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package procfs

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

type PIDStatus map[string]string

var ErrNoSuchStatusField = errors.New("no such status field")

// Reads and parses /proc/<pid>/status.
// https://man7.org/linux/man-pages/man5/proc_pid_status.5.html
func (d *PIDDir) Status() (PIDStatus, error) {
	raw, err := d.ReadFile("status")
	if err != nil {
		return nil, err
	}

	status := make(PIDStatus, 64)
	for len(raw) > 0 {
		line, rest, ok := bytes.Cut(raw, []byte{'\n'})
		if !ok {
			return nil, fmt.Errorf("status file not properly terminated: %q", raw)
		}
		name, val, ok := bytes.Cut(line, []byte{':'})
		if !ok {
			return nil, fmt.Errorf("line without colon: %q", line)
		}
		status[string(name)] = string(bytes.TrimSpace(val))
		raw = rest
	}

	return status, nil
}

// Thread group ID (i.e., Process ID).
func (s PIDStatus) ThreadGroupID() (int, error) {
	if tgid, ok := s["Tgid"]; ok {
		tgid, err := strconv.Atoi(tgid)
		return tgid, err
	}
	return 0, ErrNoSuchStatusField
}
