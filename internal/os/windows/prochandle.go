//go:build windows

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/unicode"
)

// A Windows process handle.
type ProcHandle struct {
	h windows.Handle
}

func OpenProcess(processID uint32) (_ *ProcHandle, err error) {
	const ACCESS_FLAGS = 0 |
		windows.PROCESS_QUERY_INFORMATION | // for NtQueryInformationProcess
		windows.PROCESS_VM_READ // for ReadProcessMemory

	handle, err := windows.OpenProcess(ACCESS_FLAGS, false, processID)
	if err != nil {
		// If there's no such process for the given PID, OpenProcess will return
		// an invalid parameter error. Normalize this to syscall.ESRCH.
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil, syscall.ESRCH
		}

		return nil, os.NewSyscallError("OpenProcess", err)
	}

	return &ProcHandle{handle}, nil
}

func (h *ProcHandle) Close() error {
	return os.NewSyscallError("CloseHandle", windows.CloseHandle(h.h))
}

func (h *ProcHandle) Environ() (env []string, _ error) {
	// If there's some WOW64 emulation going on, there's probably different
	// character encodings and other shenanigans involved. This code has not
	// been tested with such processes, so let's be conservative about that.
	err := ensureNoWOW64Process(h.h)
	if err != nil {
		return nil, err
	}

	envBlock, err := readEnvBlock(h.h)
	if err != nil {
		return nil, err
	}

	// The environment block uses Windows wide characters, i.e. UTF-16LE.
	// Convert this into Golang-compatible UTF-8.
	envBlock, err = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder().Bytes(envBlock)
	if err != nil {
		return nil, err
	}

	for {
		// The environment variables are separated by NUL characters.
		current, rest, ok := bytes.Cut(envBlock, []byte{0})
		if !ok {
			return nil, fmt.Errorf("variable not properly terminated: %q", envBlock)
		}
		env = append(env, string(current))

		switch len(rest) {
		default:
			envBlock = rest

		case 1: // The whole block is terminated by a NUL character as well.
			if rest[0] == 0 {
				return env, nil
			}
			fallthrough
		case 0:
			return nil, fmt.Errorf("block not properly terminated: %q", rest)
		}
	}
}

func (h *ProcHandle) Exited() (bool, error) {
	return exited(h.h)
}

func exited(handle windows.Handle) (bool, error) {
	// If the process exited, the exit code won't be STILL_ACTIVE (a.k.a STATUS_PENDING).
	// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getexitcodeprocess#remarks

	var exitCode uint32
	err := windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false, os.NewSyscallError("GetExitCodeProcess", err)
	}

	return exitCode != uint32(windows.STATUS_PENDING), nil
}

// Reads the process's environment block.
//
// The format is described here:
// https://learn.microsoft.com/en-us/windows/win32/api/processenv/nf-processenv-getenvironmentstringsw#remarks
func readEnvBlock(handle windows.Handle) ([]byte, error) {
	info, err := queryInformation(handle)
	if err != nil {
		return nil, err
	}

	// Short-circuit if the process terminated in the meantime.
	// See (*ProcHandle).exited for details.
	if info.ExitStatus != windows.STATUS_PENDING {
		return nil, os.ErrProcessDone
	}

	var peb windows.PEB // https://en.wikipedia.org/wiki/Process_Environment_Block
	err = readMemory(handle, unsafe.Pointer(info.PebBaseAddress), (*byte)(unsafe.Pointer(&peb)), unsafe.Sizeof(peb))
	if err != nil {
		return nil, err
	}

	var params windows.RTL_USER_PROCESS_PARAMETERS
	err = readMemory(handle, unsafe.Pointer(peb.ProcessParameters), (*byte)(unsafe.Pointer(&params)), unsafe.Sizeof(params))
	if err != nil {
		return nil, err
	}

	if params.EnvironmentSize == 0 {
		return nil, nil
	}

	envBlock := make([]byte, params.EnvironmentSize)
	err = readMemory(handle, params.Environment, (*byte)(unsafe.Pointer(&envBlock[0])), uintptr(len(envBlock)))
	if err != nil {
		return nil, err
	}

	return envBlock, nil
}

func queryInformation(handle windows.Handle) (*windows.PROCESS_BASIC_INFORMATION, error) {
	var data windows.PROCESS_BASIC_INFORMATION
	dataSize := unsafe.Sizeof(data)
	var bytesRead uint32
	err := windows.NtQueryInformationProcess(
		handle,
		windows.ProcessBasicInformation,
		unsafe.Pointer(&data),
		uint32(dataSize),
		&bytesRead,
	)
	if err != nil {
		return nil, os.NewSyscallError("NtQueryInformationProcess", err)
	}
	if dataSize != uintptr(bytesRead) {
		return nil, fmt.Errorf("NtQueryInformationProcess: read mismatch (%d != %d)", dataSize, bytesRead)
	}

	return &data, nil
}

func readMemory(handle windows.Handle, address unsafe.Pointer, buf *byte, size uintptr) error {
	var bytesRead uintptr
	err := windows.ReadProcessMemory(handle, uintptr(address), buf, size, &bytesRead)
	if err != nil {
		// If the process has exited by the time ReadProcessMemory is called, it
		// may return a partial copy error. Normalize this error to
		// os.ErrProcessDone. However, it is difficult to trigger this error via
		// a unit test without adding artificial, test-only code. For example,
		// adding a ten-millisecond sleep at the beginning of this method
		// reliably triggers the error.
		if errors.Is(err, windows.ERROR_PARTIAL_COPY) {
			exited, exitedErr := exited(handle)
			if exitedErr != nil {
				return errors.Join(err, exitedErr)
			}
			if exited {
				return os.ErrProcessDone
			}
		}

		return os.NewSyscallError("ReadProcessMemory", err)
	}
	if size != bytesRead {
		return fmt.Errorf("ReadProcessMemory: read mismatch (%d != %d)", size, bytesRead)
	}

	return nil
}

func ensureNoWOW64Process(handle windows.Handle) error {
	// https://learn.microsoft.com/en-us/windows/win32/sysinfo/image-file-machine-constants
	const IMAGE_FILE_MACHINE_UNKNOWN = 0 // Unknown

	// On success, returns a pointer to an IMAGE_FILE_MACHINE_* value. The value
	// will be IMAGE_FILE_MACHINE_UNKNOWN if the target process is not a WOW64
	// process; otherwise, it will identify the type of WoW process.
	// https://learn.microsoft.com/en-us/windows/win32/api/wow64apiset/nf-wow64apiset-iswow64process2#parameters
	processMachine := uint16(math.MaxUint16)

	err := windows.IsWow64Process2(handle, &processMachine, nil)
	if err != nil {
		return os.NewSyscallError("IsWow64Process2", err)
	}

	if processMachine == IMAGE_FILE_MACHINE_UNKNOWN {
		return nil
	}

	return fmt.Errorf("%w for WOW64 processes (0x%x)", errors.ErrUnsupported, processMachine)
}
