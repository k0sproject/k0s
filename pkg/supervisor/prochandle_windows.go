/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

func requestGracefulShutdown(p *os.Process) error {
	// According to https://stackoverflow.com/q/1798771/, the _only_ somewhat
	// straight-forward option is to send Ctrl+Break events to processes which
	// have been started with the CREATE_NEW_PROCESS_GROUP flag. Sending Ctrl+C
	// seems to require at least some helper process. If Ctrl+Break will
	// _actually_ trigger a graceful process shutdown is dependent of the
	// program being run. According to the above Stack Overflow question, this
	// is e.g. not the case for Python.
	//
	// Luckily, the Go runtime translates Ctrl+Break events into os.Interrupt
	// signals, and all of k0s's supervised executables are Go programs, so this
	// is mostly fine.
	err := windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(p.Pid))
	return os.NewSyscallError("GenerateConsoleCtrlEvent", err)
}
