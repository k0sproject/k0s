//go:build unix

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

func ShutdownHelperHook() {
	// no-op
}

type unixProcess struct {
	// The PID that was used when opening the process.
	// Note: Don't rely on [os.Process.Pid] here, as it's not thread safe.
	pid     int
	process *os.Process
}

func openPID(pid int) (procHandle, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	// Send "the null signal" to probe if the process actually exists.
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

// requestGracefulShutdown implements [procHandle].
func (p *unixProcess) requestGracefulShutdown() error {
	return p.process.Signal(syscall.SIGTERM)
}

// kill implements [procHandle].
func (p *unixProcess) kill() error {
	return p.process.Kill()
}

func requestGracefulShutdown(p *os.Process) error {
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	return nil
}
