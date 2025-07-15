// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/k0sproject/k0s/internal/os/windows"
)

const shutdownHelperHookMarker = "__K0S_SUPERVISOR_SHUTDOWN_HELPER"

func ShutdownHelperHook() {
	// React to command lines like "k0s shutdownHelperHookMarker <pid>"
	if len(os.Args) != 3 || os.Args[1] != shutdownHelperHookMarker {
		return
	}

	// Parse the process ID from the command line arguments.
	var processID uint32
	if parsed, err := strconv.ParseUint(os.Args[2], 10, 32); err != nil {
		fmt.Fprintln(os.Stderr, "Error: invalid process ID:", err)
		os.Exit(1)
	} else if parsed == 0 {
		fmt.Fprintln(os.Stderr, "Error: process ID may not be zero")
		os.Exit(1)
	} else {
		processID = uint32(parsed)
	}

	if err := runShutdownHelper(processID); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	}

	fmt.Fprintln(os.Stderr, "Done.")
	os.Exit(0)
}

type process struct {
	processID uint32
	handle    *windows.ProcHandle
}

func openPID(pid int) (procHandle, error) {
	if pid < 1 {
		return nil, fmt.Errorf("illegal PID: %d", pid)
	}

	processID := uint32(pid)
	handle, err := windows.OpenProcess(processID)
	if err != nil {
		return nil, err
	}

	return &process{processID, handle}, nil
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
	if err := sendCtrlBreak(p.processID); err != nil {
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
func sendCtrlBreak(processID uint32) error {
	return windows.GenerateCtrlBreakEvent(processID)
}

func runShutdownHelper(processID uint32) error {
	// Prevent this process from receiving any control events
	_, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return attachedToProcessConsole(processID, func() error {
		return windows.GenerateCtrlBreakEvent(processID)
	})
}

func attachedToProcessConsole(processID uint32, f func() error) (err error) {
	// Detach from current console
	if err := windows.FreeConsole(); err != nil {
		return err
	}
	// Re-attach to parent's console later on
	defer func() { err = errors.Join(err, windows.AttachConsole(windows.ATTACH_PARENT_PROCESS)) }()

	// Attach to the target's console
	if err := windows.AttachConsole(processID); err != nil {
		return err
	}
	// Detach from the process console later on
	defer func() { err = errors.Join(err, windows.FreeConsole()) }()

	return f()
}
