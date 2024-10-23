//go:build unix

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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type unixPID int

func newProcHandle(pid int) (procHandle, error) {
	return unixPID(pid), nil
}

func (pid unixPID) cmdline() ([]string, error) {
	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(int(pid)), "cmdline"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", syscall.ESRCH, err)
		}
		return nil, fmt.Errorf("failed to read process cmdline: %w", err)
	}

	return strings.Split(string(cmdline), "\x00"), nil
}

func (pid unixPID) environ() ([]string, error) {
	env, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(int(pid)), "environ"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", syscall.ESRCH, err)
		}
		return nil, fmt.Errorf("failed to read process environ: %w", err)
	}

	return strings.Split(string(env), "\x00"), nil
}

func (pid unixPID) terminateGracefully() error {
	return syscall.Kill(int(pid), syscall.SIGTERM)
}

func (pid unixPID) terminateForcibly() error {
	return syscall.Kill(int(pid), syscall.SIGKILL)
}
