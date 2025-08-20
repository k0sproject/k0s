// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
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

func (p *unixProcess) hasTerminated() (bool, error) {
	// Send "the null signal" to probe if the PID still exists
	// (https://www.man7.org/linux/man-pages/man3/kill.3p.html).
	if err := p.process.Signal(syscall.Signal(0)); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return true, nil
		}
		return false, err
	}

	// Checking for termination is harder than one might think when the
	// underlying os.Process may have an open pidfd, as is the case with modern
	// kernels. The "null signal" trick won't work as the pidfd will cause the
	// kernel to retain the process as a zombie for as long as it remains open.
	// We also can't call os.Process.Wait, as this only works for child
	// processes. If we had direct access to the pidfd itself, we could perform
	// the correct system calls ourselves, but the Go process API won't provide
	// it.

	// Instead, rely on the /proc filesystem once again, and check if the
	// process is a zombie.
	zombie, err := isZombie(p.pid)
	if err != nil {
		return false, err
	}

	// Do a last TOCTOU check: We now know the state of the process with the
	// referenced PID. We go back to os.Process, once again, to ensure that
	// nothing changed about the pidfd.
	if err := p.process.Signal(syscall.Signal(0)); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return true, nil
		}
		return false, err
	}

	return zombie, nil
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

func isZombie(pid int) (bool, error) {
	// https://man7.org/linux/man-pages/man5/proc_pid_stat.5.html
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false, err
	}

	// Discard the pid and comm fields: The last parenthesis marks the end of
	// the comm field, all other fields won't contain parentheses. The end of
	// comm needs to be at the fifth byte the earliest.
	if endOfComm := bytes.LastIndex(stat, []byte{')', ' '}); endOfComm < 4 {
		return false, errors.New("/proc/[pid]/stat malformed")
	} else {
		stat = stat[endOfComm+2:]
	}

	// Parse the state single character state field.
	if len(stat) < 2 || stat[1] != ' ' {
		return false, errors.New("/proc/[pid]/stat malformed")
	}

	return stat[0] == 'Z', nil
}
