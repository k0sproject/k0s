// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// openPID is not implemented on Windows.
func openPID(int) (procHandle, error) {
	return nil, syscall.EWINDOWS
}

func requestGracefulTermination(p *os.Process) error {
	// According to https://stackoverflow.com/q/1798771/, the _only_ somewhat
	// straight-forward option is to send Ctrl+Break events to processes which
	// have been started with the CREATE_NEW_PROCESS_GROUP flag. Sending Ctrl+C
	// seems to require at least some helper process. If Ctrl+Break will
	// _actually_ trigger a graceful process termination is dependent of the
	// program being run. According to the above Stack Overflow question, this
	// is e.g. not the case for Python.
	//
	// Luckily, the Go runtime translates Ctrl+Break events into os.Interrupt
	// signals, and all of k0s's supervised executables are Go programs, so this
	// is mostly fine.
	err := windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(p.Pid))
	return os.NewSyscallError("GenerateConsoleCtrlEvent", err)
}
