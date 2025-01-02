//go:build unix

/*
Copyright 2025 k0s authors

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

package config

import (
	"os/exec"
	"syscall"
)

// a simple long running background command for unix systems
func getBackgroundCommand() *exec.Cmd {
	cmd := exec.Command("/bin/sh", "-c", "sleep 9999")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Ensure the process runs in a separate process group
	}
	return cmd
}
