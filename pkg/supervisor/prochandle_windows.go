// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"fmt"
	"os"
	"syscall"

	internalwindows "github.com/k0sproject/k0s/internal/os/windows"

	"golang.org/x/sys/windows"
)

type process struct {
	pid    uint32
	handle *internalwindows.ProcHandle
}

func openPID(pid int) (procHandle, error) {
	if pid < 1 {
		return nil, fmt.Errorf("illegal PID: %d", pid)
	}

	p := uint32(pid)
	handle, err := internalwindows.OpenPID(p)
	if err != nil {
		return nil, err
	}

	return &process{p, handle}, nil
}

func (p *process) Close() error {
	return p.handle.Close()
}

// cmdline implements [procHandle].
func (p *process) cmdline() (_ []string, err error) {
	return nil, syscall.EWINDOWS
}

// environ implements [procHandle].
func (p *process) environ() ([]string, error) {
	return p.handle.Environ()
}

// requestGracefulShutdown implements [procHandle].
func (p *process) requestGracefulShutdown() error {
	if err := sendCtrlBreak(p.pid); err != nil {
		return fmt.Errorf("failed to send Ctrl+Break: %w", err)
	}

	return nil
}

// kill implements [procHandle].
func (p *process) kill() error {
	// Exit code 137 will be returned e.g. by shells when they observe child
	// process termination due to a SIGKILL. Let's simulate this for Windows.
	return p.handle.Terminate(137)
}

func requestGracefulShutdown(p *os.Process) error {
	if err := sendCtrlBreak(uint32(p.Pid)); err != nil {
		return fmt.Errorf("failed to send Ctrl+Break: %w", err)
	}

	return nil
}

// According to https://stackoverflow.com/q/1798771/, the _only_ somewhat
// straight-forward option for requesting graceful termination on Windows is to
// send Ctrl+Break events to processes which have been started with the
// CREATE_NEW_PROCESS_GROUP flag. Sending Ctrl+C seems to require at least some
// helper process. If Ctrl+Break will _actually_ trigger a graceful process
// shutdown is dependent of the program being run. According to the above Stack
// Overflow question, this is e.g. not the case for Python.
//
// Luckily, the Go runtime translates Ctrl+Break events into os.Interrupt
// signals, and all of k0s's supervised executables are Go programs, so this is
// mostly fine.
func sendCtrlBreak(pid uint32) error {
	err := windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, pid)
	return os.NewSyscallError("GenerateConsoleCtrlEvent", err)
}
