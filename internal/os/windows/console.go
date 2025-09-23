//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"os"

	"golang.org/x/sys/windows"
)

var (
	modKernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole   = modKernel32.NewProc("FreeConsole")
	procAttachConsole = modKernel32.NewProc("AttachConsole")
)

// Use the console of the parent of the current process.
//
// https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent#parameters
const ATTACH_PARENT_PROCESS = ^uint32(0) // -1

// Detaches the calling process from its console.
//
// https://learn.microsoft.com/en-us/windows/console/freeconsole
func FreeConsole() error {
	r, _, err := /* BOOL */ procFreeConsole.Call()
	if r == 0 {
		return os.NewSyscallError("FreeConsole", err)
	}
	return nil
}

// Attaches the calling process to the console of the specified process as a
// client application.
//
// https://learn.microsoft.com/en-us/windows/console/attachconsole
func AttachConsole(processID uint32) error {
	r, _, err := /* BOOL */ procAttachConsole.Call(
		/* _In_ DWORD dwProcessId */ uintptr(processID),
	)
	if r == 0 {
		return os.NewSyscallError("AttachConsole", err)
	}
	return nil
}

// Generates a Ctrl+C signal. This signal cannot be limited to a specific
// process group. All processes that share the same console as the calling
// process receive the signal, including the calling process.
//
// https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent#parameters
func GenerateCtrlCEvent() error {
	// If the process group is nonzero, GenerateConsoleCtrlEvent will succeed,
	// but the Ctrl+C signal will not be received by processes within the
	// specified process group.
	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, 0)
	return os.NewSyscallError("GenerateConsoleCtrlEvent", err)
}

// Generates a Ctrl+Break signal in all processes of the process group with the
// given processGroupID. Only those processes in the group that share the same
// console as the calling process receive the signal. In other words, if a
// process in the group creates a new console, that process does not receive the
// signal, nor do its descendants.
//
// If processGroupID is zero, the signal is generated in all processes that
// share the console of the calling process.
//
// https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent#parameters
func GenerateCtrlBreakEvent(processGroupID uint32) error {
	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, processGroupID)
	return os.NewSyscallError("GenerateConsoleCtrlEvent", err)
}
