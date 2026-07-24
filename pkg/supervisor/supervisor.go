// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Supervisor is dead simple and stupid process supervisor, just tries to keep the process running in a while-true loop
type Supervisor struct {
	Name           string
	BinPath        string
	RunDir         string
	DataDir        string
	Stdin          func() io.Reader
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
	log            logrus.FieldLogger
	mutex          sync.Mutex
	startStopMutex sync.Mutex
	stop           func(opts StopOpts) error
}

const k0sManaged = "_K0S_MANAGED=yes"

// processWaitQuit waits for a process to exit or a shut down signal.
// It returns true if shutdown is requested and an error if the shutdown failed.
func (s *Supervisor) processWaitQuit(ctx context.Context, cmd *exec.Cmd) (bool, error) {
	waitresult := make(chan error, 1)
	go func() {
		defer close(waitresult)
		err := cmd.Wait()
		defer func() { waitresult <- err }()
		if err := os.Remove(s.PidFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.WithError(err).Error("Failed to remove PID file")
		}
	}()

	select {
	case <-ctx.Done():
		cause := context.Cause(ctx)
		s.log.Debugf("Attempting to terminate supervised process (%v)", cause)
		var stopOpts StopOpts
		if stoppingErr, ok := errors.AsType[*stoppingErr](cause); ok {
			stopOpts = stoppingErr.opts
		}
		if terminated, err := s.terminateSupervisedProcess(cmd, waitresult, stopOpts); err != nil && !terminated {
			s.log.WithError(err).Error("Error while terminating process")
			return true, err
		} else if err != nil {
			s.log.WithError(err).Warn("Process terminated with errors")
		} else {
			s.log.Info("Process terminated successfully")
		}
		return true, nil

	case err, ok := <-waitresult:
		var exitErr *exec.ExitError
		state := cmd.ProcessState
		switch {
		case !ok:
			s.log.Error("Failed to wait for process: ", state)
		case errors.As(err, &exitErr):
			state = exitErr.ProcessState
			fallthrough
		case err == nil:
			s.log.Error("Process terminated unexpectedly: ", state)
		default:
			s.log.WithError(err).Error("Failed to wait for process: ", state)
		}
		return false, nil
	}
}

func (s *Supervisor) terminateSupervisedProcess(cmd *exec.Cmd, waitresult <-chan error, stopOpts StopOpts) (bool, error) {
	if timeout := stopOpts.DeferGracefulTerminationUntil; timeout != nil {
		// Termination request deferred, wait for process to finish on its own.
		s.log.Debug("Awaiting process termination")

		select {
		case err, ok := <-waitresult:
			var exitErr *exec.ExitError
			switch {
			case !ok:
				return true, errors.New("process wait result unavailable")
			case err == nil:
				return true, nil
			case errors.As(err, &exitErr):
				return true, fmt.Errorf("process terminated while awaiting deferred stop: %w", exitErr)
			default:
				return true, fmt.Errorf("failed to wait for process: %w", err)
			}
		case <-timeout:
			s.log.Debug("Timed out while waiting for process to terminate, requesting graceful termination")
		}
	}

	err := requestGracefulTermination(cmd.Process)
	switch {
	case err == nil:
		// Termination request sent, wait for process to finish.
		s.log.Debug("Awaiting graceful process termination for ", s.TimeoutStop)

		select {
		case err, ok := <-waitresult:
			var exitErr *exec.ExitError
			switch {
			case !ok:
				return true, errors.New("process wait result unavailable")
			case err == nil:
				return true, nil
			case errors.As(err, &exitErr):
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signal() == syscall.SIGTERM {
					return true, errors.New("process terminated without handling SIGTERM")
				}
				return true, fmt.Errorf("process terminated while stopping: %w", exitErr)
			default:
				return true, fmt.Errorf("failed to wait for process: %w", err)
			}

		case <-time.After(s.TimeoutStop):
			err = fmt.Errorf("timed out after %s while waiting for process to terminate", s.TimeoutStop)
		}

		return false, err

	case errors.Is(err, os.ErrProcessDone):
		// The process has finished even before the termination could be requested.
		var waitErr error
		select {
		case err, ok := <-waitresult:
			var exitErr *exec.ExitError
			state := cmd.ProcessState
			switch {
			case !ok:
				waitErr = errors.New("process wait result unavailable")
			case errors.As(err, &exitErr):
				state = exitErr.ProcessState
				fallthrough
			case err == nil:
				waitErr = errors.New(state.String())
			default:
				waitErr = fmt.Errorf("failed to wait for process: %s (%w)", state, err)
			}
		default:
			waitErr = errors.New("process state unavailable")
		}

		return true, fmt.Errorf("process terminated before graceful termination could be requested: %w", waitErr)

	default:
		// Something else went wrong
		return false, fmt.Errorf("failed to request graceful termination: %w", err)
	}
}

// Controls how supervised processes are stopped.
type StopOpts struct {
	// Delays sending a graceful termination request to the supervised process
	// until the channel is closed.
	//
	// This is useful when the process is expected to terminate on its own and
	// an immediate termination request would be premature. Once the channel is
	// closed, the supervisor reverts to the standard graceful termination
	// process.
	DeferGracefulTerminationUntil <-chan struct{}
}

type stoppingErr struct{ opts StopOpts }

func (*stoppingErr) Error() string { return "supervisor is stopping" }

// Supervise Starts supervising the given process
func (s *Supervisor) Supervise(ctx context.Context) error {
	s.startStopMutex.Lock()
	defer s.startStopMutex.Unlock()
	// check if it is already started
	if s.stop != nil {
		return errors.New("already started")
	}
	s.log = logrus.WithField("component", s.Name)
	s.PidFile = filepath.Join(s.RunDir, s.Name) + ".pid"

	if s.TimeoutStop == 0 {
		s.TimeoutStop = 5 * time.Second
	}
	if s.TimeoutRespawn == 0 {
		s.TimeoutRespawn = 5 * time.Second
	}

	if err := s.maybeCleanupPIDFile(ctx); err != nil {
		if !errors.Is(err, errors.ErrUnsupported) {
			return err
		}

		s.log.WithError(err).Warn("Old process cannot be terminated")
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	started, done := make(chan error, 1), make(chan struct{})
	var stopErr error

	go func() {
		defer close(done)

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
				if s.Stdin != nil {
					s.cmd.Stdin = s.Stdin()
				}

				// detach from the process group so children don't
				// get signals sent directly to parent.
				s.cmd.SysProcAttr = DetachAttr(s.UID, s.GID)

				const maxLogChunkLen = 16 * 1024
				s.cmd.Stdout = log.NewWriter(s.log.WithField("stream", "stdout"), logrus.InfoLevel, maxLogChunkLen)
				s.cmd.Stderr = log.NewWriter(s.log.WithField("stream", "stderr"), logrus.InfoLevel, maxLogChunkLen)

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
				if stopped, err := s.processWaitQuit(ctx, s.cmd); stopped {
					stopErr = err
					return
				}
			}

			// TODO Maybe some backoff thingy would be nice
			s.log.Infof("respawning in %s", s.TimeoutRespawn.String())

			select {
			case <-ctx.Done():
				s.log.Debugf("respawn canceled (%v)", context.Cause(ctx))
				return
			case <-time.After(s.TimeoutRespawn):
				s.log.Debug("respawning")
			}
		}
	}()

	if err := <-started; err != nil {
		cancel(err)
		<-done
		return err
	}

	s.stop = func(opts StopOpts) error {
		cancel(&stoppingErr{opts})
		<-done
		return stopErr
	}
	return nil
}

// Stops the supervised process using the default stop behavior.
func (s *Supervisor) Stop() error {
	return s.StopWith(StopOpts{})
}

// Stops the supervised process using the provided options.
func (s *Supervisor) StopWith(opts StopOpts) error {
	s.startStopMutex.Lock()
	defer s.startStopMutex.Unlock()
	if s.stop == nil {
		return errors.New("not started")
	}

	err := s.stop(opts)
	s.stop = nil
	return err
}

// Checks if the process referenced in the PID file is a k0s-managed process.
// If so, requests graceful termination and waits for the process to terminate.
//
// The PID file itself is not removed here; that is handled by the caller.
func (s *Supervisor) maybeCleanupPIDFile(ctx context.Context) error {
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

	return s.cleanupPID(ctx, p)
}

// Prepare the env for exec:
// - handle component specific env
// - inject k0s embedded bins into path
func getEnv(dataDir, component string, keepEnvPrefix bool) []string {
	env := os.Environ()
	componentPrefix := strings.ToUpper(component) + "_"

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
			env[i] = "PATH=" + dir.PathListJoin(filepath.Join(dataDir, "bin"), v)
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
