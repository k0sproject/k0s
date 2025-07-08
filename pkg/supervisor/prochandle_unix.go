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

type unixProcess struct {
	// The PID that was used when opening the process.
	// Note: Don't rely on [os.Process.Pid] here, as it's not thread safe.
	pid     int
	process *os.Process
}

func openPID(pid int) (procHandle, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	return &unixProcess{pid, process}, nil
}

func (p *unixProcess) Close() error {
	return p.process.Release()
}

// cmdline implements [procHandle].
func (p *unixProcess) cmdline() ([]string, error) {
	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(p.pid), "cmdline"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", syscall.ESRCH, err)
		}
		return nil, fmt.Errorf("failed to read process cmdline: %w", err)
	}

	return strings.Split(string(cmdline), "\x00"), nil
}

// environ implements [procHandle].
func (p *unixProcess) environ() ([]string, error) {
	env, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(p.pid), "environ"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", syscall.ESRCH, err)
		}
		return nil, fmt.Errorf("failed to read process environ: %w", err)
	}

	return strings.Split(string(env), "\x00"), nil
}

// requestGracefulShutdown implements [procHandle].
func (p *unixProcess) requestGracefulShutdown() error {
	return p.process.Signal(syscall.SIGTERM)
}

// kill implements [procHandle].
func (p *unixProcess) kill() error {
	return p.process.Kill()
}

func requestGracefulShutdown(p *os.Process) error {
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	return nil
}
