/*
Copyright 2020 k0s authors

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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Supervisor is dead simple and stupid process supervisor, just tries to keep the process running in a while-true loop
type Supervisor struct {
	Name           string
	BinPath        string
	RunDir         string
	DataDir        string
	Args           []string
	PidFile        string
	UID            int
	GID            int
	TimeoutStop    time.Duration
	TimeoutRespawn time.Duration
	// For those components having env prefix convention such as ETCD_xxx, we should keep the prefix.
	KeepEnvPrefix bool
	// A function to clean some leftovers before starting or restarting the supervised process
	CleanBeforeFn func() error

	cmd            *exec.Cmd
	done           chan bool
	log            logrus.FieldLogger
	mutex          sync.Mutex
	startStopMutex sync.Mutex
	cancel         context.CancelFunc
}

const k0sManaged = "_K0S_MANAGED=yes"

// processWaitQuit waits for a process to exit or a shut down signal
// returns true if shutdown is requested
func (s *Supervisor) processWaitQuit(ctx context.Context) bool {
	waitresult := make(chan error)
	go func() {
		waitresult <- s.cmd.Wait()
	}()

	defer os.Remove(s.PidFile)

	select {
	case <-ctx.Done():
		for {
			if runtime.GOOS == "windows" {
				// Graceful shutdown not implemented on Windows. This requires
				// attaching to the target process's console and generating a
				// CTRL+BREAK (or CTRL+C) event. Since a process can only be
				// attached to a single console at a time, this would require
				// k0s to detach from its own console, which is definitely not
				// something that k0s wants to do. There might be ways to do
				// this by generating the event via a separate helper process,
				// but that's left open here as a TODO.
				// https://learn.microsoft.com/en-us/windows/console/freeconsole
				// https://learn.microsoft.com/en-us/windows/console/attachconsole
				// https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
				// https://learn.microsoft.com/en-us/windows/console/ctrl-c-and-ctrl-break-signals
				s.log.Infof("Killing pid %d", s.cmd.Process.Pid)
				if err := s.cmd.Process.Kill(); err != nil {
					s.log.Warnf("Failed to kill pid %d: %s", s.cmd.Process.Pid, err)
				}
			} else {
				s.log.Infof("Shutting down pid %d", s.cmd.Process.Pid)
				if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
					s.log.Warnf("Failed to send SIGTERM to pid %d: %s", s.cmd.Process.Pid, err)
				}
			}
			select {
			case <-time.After(s.TimeoutStop):
				continue
			case <-waitresult:
				return true
			}
		}
	case err := <-waitresult:
		if err != nil {
			s.log.WithError(err).Warn("Failed to wait for process")
		} else {
			s.log.Warnf("Process exited: %s", s.cmd.ProcessState)
		}
	}
	return false
}

// Supervise Starts supervising the given process
func (s *Supervisor) Supervise() error {
	s.startStopMutex.Lock()
	defer s.startStopMutex.Unlock()
	// check if it is already started
	if s.cancel != nil {
		s.log.Warn("Already started")
		return nil
	}
	s.log = logrus.WithField("component", s.Name)
	s.PidFile = path.Join(s.RunDir, s.Name) + ".pid"
	if err := dir.Init(s.RunDir, constant.RunDirMode); err != nil {
		s.log.Warnf("failed to initialize dir: %v", err)
		return err
	}

	if s.TimeoutStop == 0 {
		s.TimeoutStop = 5 * time.Second
	}
	if s.TimeoutRespawn == 0 {
		s.TimeoutRespawn = 5 * time.Second
	}

	if err := s.maybeKillPidFile(); err != nil {
		if !errors.Is(err, errors.ErrUnsupported) {
			return err
		}

		s.log.WithError(err).Warn("Old process cannot be terminated")
	}

	var ctx context.Context
	ctx, s.cancel = context.WithCancel(context.Background())
	started := make(chan error)
	s.done = make(chan bool)

	go func() {
		defer func() {
			close(s.done)
		}()

		s.log.Info("Starting to supervise")
		restarts := 0
		for {
			s.mutex.Lock()

			var err error
			if s.CleanBeforeFn != nil {
				err = s.CleanBeforeFn()
			}
			if err != nil {
				s.log.Warnf("Failed to clean before running the process %s: %s", s.BinPath, err)
			} else {
				s.cmd = exec.Command(s.BinPath, s.Args...)
				s.cmd.Dir = s.DataDir
				s.cmd.Env = getEnv(s.DataDir, s.Name, s.KeepEnvPrefix)

				// detach from the process group so children don't
				// get signals sent directly to parent.
				s.cmd.SysProcAttr = DetachAttr(s.UID, s.GID)

				const maxLogChunkLen = 16 * 1024
				s.cmd.Stdout = &logWriter{
					log: s.log.WithField("stream", "stdout"),
					buf: make([]byte, maxLogChunkLen),
				}
				s.cmd.Stderr = &logWriter{
					log: s.log.WithField("stream", "stderr"),
					buf: make([]byte, maxLogChunkLen),
				}

				err = s.cmd.Start()
			}
			s.mutex.Unlock()
			if err != nil {
				s.log.Warnf("Failed to start: %s", err)
				if restarts == 0 {
					started <- err
					return
				}
			} else {
				err := os.WriteFile(s.PidFile, []byte(strconv.Itoa(s.cmd.Process.Pid)+"\n"), constant.PidFileMode)
				if err != nil {
					s.log.Warnf("Failed to write file %s: %v", s.PidFile, err)
				}
				if restarts == 0 {
					s.log.Infof("Started successfully, go nuts pid %d", s.cmd.Process.Pid)
					started <- nil
				} else {
					s.log.Infof("Restarted (%d)", restarts)
				}
				restarts++
				if s.processWaitQuit(ctx) {
					return
				}
			}

			// TODO Maybe some backoff thingy would be nice
			s.log.Infof("respawning in %s", s.TimeoutRespawn.String())

			select {
			case <-ctx.Done():
				s.log.Debug("respawn cancelled")
				return
			case <-time.After(s.TimeoutRespawn):
				s.log.Debug("respawning")
			}
		}
	}()
	return <-started
}

// Stop stops the supervised
func (s *Supervisor) Stop() {
	s.startStopMutex.Lock()
	defer s.startStopMutex.Unlock()
	if s.cancel == nil || s.log == nil {
		s.log.Warn("Not started")
		return
	}
	s.log.Debug("Sending stop message")

	s.cancel()
	s.cancel = nil
	s.log.Debug("Waiting for stopping is done")
	if s.done != nil {
		<-s.done
	}
}

// maybeKillPidFile checks kills the process in the pidFile if it's has
// the same binary as the supervisor's and also checks that the env
// `_KOS_MANAGED=yes`. This function does not delete the old pidFile as
// this is done by the caller.
func (s *Supervisor) maybeKillPidFile() error {
	pid, err := os.ReadFile(s.PidFile)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read PID file %s: %w", s.PidFile, err)
	}

	p, err := strconv.Atoi(strings.TrimSuffix(string(pid), "\n"))
	if err != nil {
		return fmt.Errorf("failed to parse PID file %s: %w", s.PidFile, err)
	}

	ph, err := newProcHandle(p)
	if err != nil {
		return fmt.Errorf("cannot interact with PID %d from PID file %s: %w", p, s.PidFile, err)
	}

	if err := s.killProcess(ph); err != nil {
		return fmt.Errorf("failed to kill PID %d from PID file %s: %w", p, s.PidFile, err)
	}

	return nil
}

const exitCheckInterval = 200 * time.Millisecond

// Tries to terminate a process gracefully. If it's still running after
// s.TimeoutStop, the process is forcibly terminated.
func (s *Supervisor) killProcess(ph procHandle) error {
	// Kill the process pid
	deadlineTicker := time.NewTicker(s.TimeoutStop)
	defer deadlineTicker.Stop()
	checkTicker := time.NewTicker(exitCheckInterval)
	defer checkTicker.Stop()

Loop:
	for {
		select {
		case <-checkTicker.C:
			shouldKill, err := s.shouldKillProcess(ph)
			if err != nil {
				return err
			}
			if !shouldKill {
				return nil
			}

			err = ph.terminateGracefully()
			if errors.Is(err, syscall.ESRCH) {
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to terminate gracefully: %w", err)
			}
		case <-deadlineTicker.C:
			break Loop
		}
	}

	shouldKill, err := s.shouldKillProcess(ph)
	if err != nil {
		return err
	}
	if !shouldKill {
		return nil
	}

	err = ph.terminateForcibly()
	if errors.Is(err, syscall.ESRCH) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to terminate forcibly: %w", err)
	}
	return nil
}

func (s *Supervisor) shouldKillProcess(ph procHandle) (bool, error) {
	// only kill process if it has the expected cmd
	if cmd, err := ph.cmdline(); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return false, nil
		}
		return false, err
	} else if len(cmd) > 0 && cmd[0] != s.BinPath {
		return false, nil
	}

	//only kill process if it has the _KOS_MANAGED env set
	if env, err := ph.environ(); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return false, nil
		}
		return false, err
	} else if !slices.Contains(env, k0sManaged) {
		return false, nil
	}

	return true, nil
}

// Prepare the env for exec:
// - handle component specific env
// - inject k0s embedded bins into path
func getEnv(dataDir, component string, keepEnvPrefix bool) []string {
	env := os.Environ()
	componentPrefix := fmt.Sprintf("%s_", strings.ToUpper(component))

	// put the component specific env vars in the front.
	sort.Slice(env, func(i, j int) bool { return strings.HasPrefix(env[i], componentPrefix) })

	overrides := map[string]struct{}{}
	i := 0
	for _, e := range env {
		kv := strings.SplitN(e, "=", 2)
		k, v := kv[0], kv[1]
		// if there is already a correspondent component specific env, skip it.
		if _, ok := overrides[k]; ok {
			continue
		}
		if strings.HasPrefix(k, componentPrefix) {
			var shouldOverride bool
			k1 := strings.TrimPrefix(k, componentPrefix)
			switch k1 {
			// always override proxy env
			case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY":
				shouldOverride = true
			default:
				if !keepEnvPrefix {
					shouldOverride = true
				}
			}
			if shouldOverride {
				k = k1
				overrides[k] = struct{}{}
			}
		}
		switch k {
		case "PATH":
			env[i] = fmt.Sprintf("PATH=%s", dir.PathListJoin(path.Join(dataDir, "bin"), v))
		default:
			env[i] = fmt.Sprintf("%s=%s", k, v)
		}
		i++
	}
	env = append([]string{k0sManaged}, env...)
	i++

	return env[:i]
}

// GetProcess returns the last started process
func (s *Supervisor) GetProcess() *os.Process {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Process
}
