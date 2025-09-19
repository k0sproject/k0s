//go:build linux || windows

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func (s *Supervisor) cleanupPID(ctx context.Context, pid int) error {
	ph, err := openPID(pid)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil // no such process, nothing to cleanup
		}
		return fmt.Errorf("cannot open process for PID %d from PID file %s: %w", pid, s.PidFile, err)
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

	if err := s.terminateAndWait(ctx, ph); err != nil {
		return fmt.Errorf("while waiting for termination of PID %d from PID file %s: %w", pid, s.PidFile, err)
	}

	return nil
}

// Tries to gracefully terminate a process and waits for it to exit. If the
// process is still running after several attempts, it returns an error instead
// of forcefully killing the process.
func (s *Supervisor) terminateAndWait(ctx context.Context, ph procHandle) error {
	if err := ph.requestGracefulTermination(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("failed to request graceful termination: %w", err)
	}

	errTimeout := errors.New("process did not terminate in time")
	ctx, cancel := context.WithTimeoutCause(ctx, s.TimeoutStop, errTimeout)
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

// A handle to a running process. May be used to inspect the process properties
// and terminate it.
type procHandle interface {
	io.Closer

	// Checks whether the process has terminated.
	hasTerminated() (bool, error)

	// Reads and returns the process's command line.
	cmdline() ([]string, error)

	// Reads and returns the process's environment.
	environ() ([]string, error)

	// Requests graceful process termination.
	requestGracefulTermination() error
}
