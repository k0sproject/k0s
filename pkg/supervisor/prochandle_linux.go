// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type unixProcess struct {
	// The PID that was used when opening the process.
	// Note: Don't rely on [os.Process.Pid] here, as it's not thread safe.
	pid     int
	process *os.Process
}

func openPID(pid int) (_ procHandle, err error) {
	var process *os.Process
	process, err = os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, process.Release())
		}
	}()

	// FindProcess won't return an error if no such process exists.
	// Be fail-fast and check it directly by sending "the null signal".
	// https://www.man7.org/linux/man-pages/man3/kill.3p.html
	if err := process.Signal(syscall.Signal(0)); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil, syscall.ESRCH
		}
	}

	return &unixProcess{pid, process}, nil
}

func (p *unixProcess) Close() error {
	return p.process.Release()
}

// cmdline implements [procHandle].
func (p *unixProcess) cmdline() ([]string, error) {
	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(p.pid), "cmdline"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", os.ErrProcessDone, err)
		}
		return nil, fmt.Errorf("failed to read process cmdline: %w", err)
	}

	return strings.Split(string(cmdline), "\x00"), nil
}

// environ implements [procHandle].
func (p *unixProcess) environ() ([]string, error) {
	env, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(p.pid), "environ"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", os.ErrProcessDone, err)
		}
		return nil, fmt.Errorf("failed to read process environ: %w", err)
	}

	return strings.Split(string(env), "\x00"), nil
}

// requestGracefulTermination implements [procHandle].
func (p *unixProcess) requestGracefulTermination() error {
	return requestGracefulTermination(p.process)
}

// kill implements [procHandle].
func (p *unixProcess) kill() error {
	return p.process.Kill()
}
