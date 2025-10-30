// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"

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
			s.log.Debug("Requesting graceful termination")
			if err := requestGracefulTermination(s.cmd.Process); err != nil {
				if errors.Is(err, os.ErrProcessDone) {
					s.log.Info("Failed to request graceful termination: process has already terminated")
				} else {
					s.log.WithError(err).Error("Failed to request graceful termination")
				}
			} else {
				s.log.Info("Requested graceful termination")
			}
			select {
			case <-time.After(s.TimeoutStop):
				continue
			case err := <-waitresult:
				if err != nil {
					s.log.WithError(err).Error("Failed to wait for process")
				} else {
					s.log.Info("Process exited: ", s.cmd.ProcessState)
				}
				return true
			}
		}
	case err := <-waitresult:
		if err != nil {
			s.log.WithError(err).Warn("Failed to wait for process")
		} else {
			s.log.Warnf("Process exited: ", s.cmd.ProcessState)
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
	s.PidFile = filepath.Join(s.RunDir, s.Name) + ".pid"

	if s.TimeoutStop == 0 {
		s.TimeoutStop = 5 * time.Second
	}
	if s.TimeoutRespawn == 0 {
		s.TimeoutRespawn = 5 * time.Second
	}

	if err := s.maybeCleanupPIDFile(); err != nil {
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
				if s.Stdin != nil {
					s.cmd.Stdin = s.Stdin()
				}

				// detach from the process group so children don't
				// get signals sent directly to parent.
				s.cmd.SysProcAttr = DetachAttr(s.UID, s.GID)

				const maxLogChunkLen = 16 * 1024
				s.cmd.Stdout = log.NewWriter(s.log.WithField("stream", "stdout"), maxLogChunkLen)
				s.cmd.Stderr = log.NewWriter(s.log.WithField("stream", "stderr"), maxLogChunkLen)

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
				s.log.Debug("respawn canceled")
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

// Checks if the process referenced in the PID file is a k0s-managed process.
// If so, requests graceful termination and waits for the process to terminate.
//
// The PID file itself is not removed here; that is handled by the caller.
func (s *Supervisor) maybeCleanupPIDFile() error {
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

	ph, err := openPID(p)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil // no such process, nothing to cleanup
		}
		return fmt.Errorf("cannot interact with PID %d from PID file %s: %w", p, s.PidFile, err)
	}
	defer ph.Close()

	if managed, err := s.isK0sManaged(ph); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	} else if !managed {
		return nil
	}

	if err := s.terminateAndWait(ph); err != nil {
		return fmt.Errorf("while waiting for termination of PID %d from PID file %s: %w", p, s.PidFile, err)
	}

	return nil
}

// Tries to gracefully terminate a process and waits for it to exit. If the
// process is still running after several attempts, it returns an error instead
// of forcefully killing the process.
func (s *Supervisor) terminateAndWait(ph procHandle) error {
	if err := ph.requestGracefulTermination(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("failed to request graceful termination: %w", err)
	}

	errTimeout := errors.New("process did not terminate in time")
	ctx, cancel := context.WithTimeoutCause(context.TODO(), s.TimeoutStop, errTimeout)
	defer cancel()
	return s.awaitTermination(ctx, ph)
}

// Checks if the process handle refers to a k0s-managed process. A process is
// considered k0s-managed if:
//   - The executable path matches.
//   - The process environment contains `_K0S_MANAGED=yes`.
func (s *Supervisor) isK0sManaged(ph procHandle) (bool, error) {
	if cmd, err := ph.cmdline(); err != nil {
		// Only error out if the error doesn't indicate that getting the command
		// line is unsupported. In that case, ignore the error and proceed to
		// the environment check.
		if !errors.Is(err, errors.ErrUnsupported) {
			return false, err
		}
	} else if len(cmd) > 0 && cmd[0] != s.BinPath {
		return false, nil
	}

	if env, err := ph.environ(); err != nil {
		return false, err
	} else if !slices.Contains(env, k0sManaged) {
		return false, nil
	}

	return true, nil
}

func (s *Supervisor) awaitTermination(ctx context.Context, ph procHandle) error {
	s.log.Debug("Polling for process termination")
	backoff := wait.Backoff{
		Duration: 25 * time.Millisecond,
		Cap:      3 * time.Second,
		Steps:    math.MaxInt32,
		Factor:   1.5,
		Jitter:   0.1,
	}

	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(context.Context) (bool, error) {
		return ph.hasTerminated()
	}); err != nil {
		if err == ctx.Err() { //nolint:errorlint // the equal check is intended
			return context.Cause(ctx)
		}

		return err
	}

	return nil
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
