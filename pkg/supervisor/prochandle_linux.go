// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/k0sproject/k0s/internal/os/linux"
	"github.com/k0sproject/k0s/internal/os/linux/procfs"
	"github.com/k0sproject/k0s/internal/os/unix"
)

type unixProcess struct {
	pid    int
	pidDir *unix.Dir
}

func openPID(pid int) (_ *unixProcess, err error) {
	p := &unixProcess{pid: pid}
	p.pidDir, err = procfs.OpenPID(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, syscall.ESRCH
		}
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.Close())
		}
	}()

	// The dir is open. It might refer to a thread, though.
	// Check if the thread group ID is the process ID.
	if status, err := p.dir().Status(); err != nil {
		return nil, err
	} else if tgid, err := status.ThreadGroupID(); err != nil {
		return nil, fmt.Errorf("failed to get thread group ID: %w", err)
	} else if tgid != pid {
		return nil, fmt.Errorf("%w (thread group ID is %d)", syscall.ESRCH, tgid)
	}

	return p, nil
}

func (p *unixProcess) Close() error {
	return p.pidDir.Close()
}

func (p *unixProcess) hasTerminated() (bool, error) {
	// Checking for termination is harder than one might think when there are
	// open file descriptors to that process. The "null signal" trick won't work
	// because the process remains a zombie as long as there are open file
	// descriptors to it. Rely on the proc filesystem once again to check if the
	// process has terminated or is a zombie.
	state, err := p.dir().State()
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return true, nil
		}
		return false, err
	}

	return state == procfs.PIDStateZombie, nil
}

// cmdline implements [procHandle].
func (p *unixProcess) cmdline() ([]string, error) {
	cmdline, err := p.dir().Cmdline()
	if errors.Is(err, syscall.ESRCH) {
		return nil, os.ErrProcessDone
	}
	return cmdline, err
}

// environ implements [procHandle].
func (p *unixProcess) environ() ([]string, error) {
	env, err := p.dir().Environ()
	if errors.Is(err, syscall.ESRCH) {
		return nil, os.ErrProcessDone
	}
	return env, err
}

// requestGracefulTermination implements [procHandle].
func (p *unixProcess) requestGracefulTermination() error {
	if err := linux.SendSignal(p.pidDir, syscall.SIGTERM); errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	} else if !errors.Is(err, errors.ErrUnsupported) {
		return err
	}

	if err := syscall.Kill(p.pid, syscall.SIGTERM); errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	} else {
		return err
	}
}

// awaitTermination implements [procHandle].
func (p *unixProcess) awaitTermination(ctx context.Context) (err error) {
	// The /proc/<pid> dirfds are not pollable. Use a pidfd, if available. This
	// is the only way that doesn't involve userspace polling using timeouts and
	// such, but requires at least Linux 5.3.
	proc, err := linux.OpenProcess(p.pid)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, proc.Close()) }()

	// Since the process has been opened via its PID, there might have been a
	// race. Check if the process is still alive. If it is, then it's guaranteed
	// that the PID hasn't been recycled in the meantime and both pidFD and proc
	// are referring to the same process.
	if terminated, err := p.hasTerminated(); err != nil || terminated {
		return err
	}

	return proc.Wait(ctx)
}

func (p *unixProcess) dir() *procfs.PIDDir {
	return &procfs.PIDDir{FS: p.pidDir}
}
