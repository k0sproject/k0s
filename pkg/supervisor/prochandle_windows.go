// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	internalio "github.com/k0sproject/k0s/internal/io"
	"github.com/k0sproject/k0s/internal/os/windows"

	"github.com/sirupsen/logrus"
)

const terminationHelperHookMarker = "__K0S_SUPERVISOR_TERMINATION_HELPER"

func TerminationHelperHook() {
	// React to command lines like "k0s terminationHelperHookMarker <pid>"
	if len(os.Args) != 3 || os.Args[1] != terminationHelperHookMarker {
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

	if err := runTerminationHelper(processID); err != nil {
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

// hasTerminated implements procHandle.
func (p *process) hasTerminated() (bool, error) {
	return p.handle.Exited()
}

// cmdline implements [procHandle].
func (p *process) cmdline() (_ []string, err error) {
	return nil, syscall.EWINDOWS
}

// environ implements [procHandle].
func (p *process) environ() ([]string, error) {
	return p.handle.Environ()
}

// requestGracefulTermination implements [procHandle].
//
// Windows requires that processes be attached to the same console in order to
// send console control events to each other. A process can only be attached to
// one console at a time, and k0s cannot simply detach from its own console to
// send these events.
//
// Instead, spawn a new k0s process in a special "termination helper mode" to to
// send the console events. This helper process can freely detach and reattach
// consoles without affecting the main k0s process.
//
// The termination helper process sends Ctrl+Break events because they can be
// targeted at a specific process group. In contrast, Ctrl+C events are
// broadcasted to _all_ processes attached to the terminal, which is not
// desirable for k0s's use case. Whether Ctrl+Break _actually_ triggers a
// graceful process termination depends on the program being run. Luckily, the Go
// runtime translates Ctrl+Break events into os.Interrupt signals, and all of
// k0s's supervised executables are Go programs, so this is mostly fine.
func (p *process) requestGracefulTermination() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine path to k0s executable: %w", err)
	}

	cmd := exec.Command(exe, terminationHelperHookMarker, strconv.FormatUint(uint64(p.processID), 10))
	cmd.Env = []string{}

	var (
		mu  sync.Mutex
		out bytes.Buffer
	)
	w := internalio.WriterFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return out.Write(p)
	})

	cmd.Stdout, cmd.Stderr = w, w
	result := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn termination helper process: %w", err)
	}
	go func() { result <- cmd.Wait() }()

	helperTimeout := 30 * time.Second
	logrus.Debug("Waiting for supervisor termination helper process for ", helperTimeout)
	select {
	case err := <-result:
		if err == nil {
			return nil
		}
		return fmt.Errorf("termination helper process failed: %w (%q)", err, bytes.TrimSpace(out.Bytes()))

	case <-time.After(helperTimeout):
		err := cmd.Process.Kill()
		mu.Lock()
		defer mu.Unlock()
		return errors.Join(fmt.Errorf("timed out while waiting for termination helper process to terminate: %q", bytes.TrimSpace(out.Bytes())), err)
	}
}

func requestGracefulTermination(p *os.Process) error {
	if err := sendCtrlBreak(uint32(p.Pid)); err != nil {
		return fmt.Errorf("failed to send Ctrl+Break: %w", err)
	}

	return nil
}

// According to https://stackoverflow.com/q/1798771/, the _only_ somewhat
// straight-forward option for requesting graceful termination on Windows is to
// send Ctrl+Break events to processes which have been started with the
// CREATE_NEW_PROCESS_GROUP flag. In contrast, Ctrl+C events are broadcasted to
// _all_ processes attached to the terminal, which is not desirable for k0s's
// use case: It would send a graceful termination request to itself.
//
// Whether Ctrl+Break _actually_ triggers a graceful process termination depends on
// the program being run. According to the above Stack Overflow question, this
// is e.g. not the case for Python. Luckily, the Go runtime translates
// Ctrl+Break events into os.Interrupt signals, and all of k0s's supervised
// executables are Go programs, so this is mostly fine.
func sendCtrlBreak(processID uint32) error {
	return windows.GenerateCtrlBreakEvent(processID)
}

func runTerminationHelper(processID uint32) error {
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
