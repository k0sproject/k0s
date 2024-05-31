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
	"time"
)

const (
	exitCheckInterval = 200 * time.Millisecond
)

// killPid signals SIGTERM to a PID and if it's still running after
// s.TimeoutStop sends SIGKILL.
func (s *Supervisor) killPid(pid int, check <-chan time.Time, deadline <-chan time.Time) error {
	if s.KillFunction == nil {
		s.KillFunction = syscall.Kill
	}
	// Kill the process pid
	if deadline == nil {
		deadlineTicker := time.NewTicker(s.TimeoutStop)
		deadline = deadlineTicker.C
		defer deadlineTicker.Stop()
	}
	if check == nil {
		checkTicker := time.NewTicker(exitCheckInterval)
		check = checkTicker.C
		defer checkTicker.Stop()

	}

	// Using two tickers is not very elegant but makes testing easier...
Loop:
	for {
		select {
		case <-check:
			shouldKill, err := s.shouldKillProcess(pid)
			if err != nil {
				return err
			}
			if !shouldKill {
				return nil
			}

			err = s.KillFunction(pid, syscall.SIGTERM)
			if errors.Is(err, syscall.ESRCH) {
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to send SIGTERM: %w", err)
			}
		case <-deadline:
			break Loop
		}
	}

	shouldKill, err := s.shouldKillProcess(pid)
	if err != nil {
		return err
	}
	if !shouldKill {
		return nil
	}

	err = s.KillFunction(pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to send SIGKILL: %w", err)
	}
	return nil
}

// maybeKillPidFile checks kills the process in the pidFile if it's has
// the same binary as the supervisor's and also checks that the env
// `_KOS_MANAGED=yes`. This function does not delete the old pidFile as
// this is done by the caller.
// The tickers are used for testing purposes, otherwise set them to nil.
func (s *Supervisor) maybeKillPidFile(check <-chan time.Time, deadline <-chan time.Time) error {
	if s.ProcFSPath == "" {
		s.ProcFSPath = "/proc"
	}

	pid, err := os.ReadFile(s.PidFile)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read pid file %s: %w", s.PidFile, err)
	}

	p, err := strconv.Atoi(strings.TrimSuffix(string(pid), "\n"))
	if err != nil {
		return fmt.Errorf("failed to parse pid file %s: %w", s.PidFile, err)
	}

	if err := s.killPid(p, check, deadline); err != nil {
		return fmt.Errorf("failed to kill process with PID %d: %w", p, err)
	}

	return nil
}

func (s *Supervisor) shouldKillProcess(pid int) (bool, error) {
	cmdline, err := os.ReadFile(filepath.Join(s.ProcFSPath, strconv.Itoa(pid), "cmdline"))
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to read process cmdline: %w", err)
	}

	// only kill process if it has the expected cmd
	cmd := strings.Split(string(cmdline), "\x00")
	if cmd[0] != s.BinPath {
		return false, nil
	}

	//only kill process if it has the _KOS_MANAGED env set
	env, err := os.ReadFile(filepath.Join(s.ProcFSPath, strconv.Itoa(pid), "environ"))
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to read process environ: %w", err)
	}

	for _, e := range strings.Split(string(env), "\x00") {
		if e == k0sManaged {
			return true, nil
		}
	}
	return false, nil
}
