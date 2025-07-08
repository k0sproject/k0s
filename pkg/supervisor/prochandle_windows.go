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
	"syscall"
)

// openPID is not implemented on Windows.
func openPID(int) (procHandle, error) {
	return nil, syscall.EWINDOWS
}

func requestGracefulShutdown(p *os.Process) error {
	// Graceful shutdown not implemented on Windows. This requires attaching to
	// the target process's console and generating a CTRL+BREAK (or CTRL+C)
	// event. Since a process can only be attached to a single console at a
	// time, this would require k0s to detach from its own console, which is
	// definitely not something that k0s wants to do. There might be ways to do
	// this by generating the event via a separate helper process, but that's
	// left open here as a TODO.
	// https://learn.microsoft.com/en-us/windows/console/freeconsole
	// https://learn.microsoft.com/en-us/windows/console/attachconsole
	// https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
	// https://learn.microsoft.com/en-us/windows/console/ctrl-c-and-ctrl-break-signals
	if err := p.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	return nil
}
