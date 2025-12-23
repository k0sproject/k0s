// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/testutil/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SupervisorTest struct {
	expectedErrMsg string
	proc           Supervisor
}

func TestSupervisorStart(t *testing.T) {
	sleep := selectCmd(t,
		cmd{"sleep", []string{"60"}},
		cmd{"powershell", []string{"-noprofile", "-noninteractive", "-command", "Start-Sleep -Seconds 60"}},
	)

	fail := selectCmd(t,
		cmd{"false", []string{}},
		cmd{"sh", []string{"-c", "exit 1"}},
		cmd{"powershell", []string{"-noprofile", "-noninteractive", "-command", "exit 1"}},
	)

	var testSupervisors = []*SupervisorTest{
		{
			proc: Supervisor{
				Name:    "supervisor-test-sleep",
				BinPath: sleep.binPath,
				Args:    sleep.binArgs,
				RunDir:  t.TempDir(),
			},
		},
		{
			proc: Supervisor{
				Name:    "supervisor-test-fail",
				BinPath: fail.binPath,
				Args:    fail.binArgs,
				RunDir:  t.TempDir(),
			},
		},
		{
			expectedErrMsg: "exec",
			proc: Supervisor{
				Name:    "supervisor-test-non-executable",
				BinPath: t.TempDir(),
				RunDir:  t.TempDir(),
			},
		},
	}

	for _, s := range testSupervisors {
		t.Run(s.proc.Name, func(t *testing.T) {
			err := s.proc.Supervise(t.Context())
			if s.expectedErrMsg != "" {
				assert.ErrorContains(t, err, s.expectedErrMsg)
				assert.ErrorContains(t, s.proc.Stop(), "not started")
			} else {
				assert.NoError(t, err, "Failed to start")
				assert.NoError(t, s.proc.Stop())
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	// Cleanup the environment variables before the test to ensure a clean slate.
	// t.Setenv is used to reset each variable, and it automatically restores the
	// original environment after the test finishes.
	oldEnv := os.Environ()
	for _, e := range oldEnv {
		key, _, _ := strings.Cut(e, "=")
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	t.Setenv("k3", "v3")
	t.Setenv("PATH", "/bin")
	t.Setenv("k2", "v2")
	t.Setenv("FOO_k3", "foo_v3")
	t.Setenv("k4", "v4")
	t.Setenv("FOO_k2", "foo_v2")
	t.Setenv("FOO_HTTPS_PROXY", "a.b.c:1080")
	t.Setenv("HTTPS_PROXY", "1.2.3.4:8888")
	t.Setenv("k1", "v1")
	t.Setenv("FOO_PATH", "/usr/local/bin")

	expected := []string{
		"HTTPS_PROXY=a.b.c:1080",
		"PATH=" + dir.PathListJoin(
			filepath.FromSlash("/var/lib/k0s/bin"),
			"/usr/local/bin",
		),
		"_K0S_MANAGED=yes",
		"k1=v1",
		"k2=foo_v2",
		"k3=foo_v3",
		"k4=v4",
	}
	actual := getEnv(filepath.FromSlash("/var/lib/k0s"), "foo", false)
	assert.ElementsMatch(t, expected, actual)

	expected = []string{
		"FOO_PATH=/usr/local/bin",
		"FOO_k2=foo_v2",
		"FOO_k3=foo_v3",
		"HTTPS_PROXY=a.b.c:1080",
		"PATH=" + dir.PathListJoin(
			filepath.FromSlash("/var/lib/k0s/bin"),
			"/bin",
		),
		"_K0S_MANAGED=yes",
		"k1=v1",
		"k2=v2",
		"k3=v3",
		"k4=v4",
	}
	actual = getEnv(filepath.FromSlash("/var/lib/k0s"), "foo", true)
	assert.ElementsMatch(t, expected, actual)
}

func TestRespawn(t *testing.T) {
	pingPong := pingpong.New(t)

	s := Supervisor{
		Name:           t.Name(),
		BinPath:        pingPong.BinPath(),
		RunDir:         t.TempDir(),
		Args:           pingPong.BinArgs(),
		TimeoutStop:    1 * time.Minute,
		TimeoutRespawn: 1 * time.Millisecond,
	}
	require.NoError(t, s.Supervise(t.Context()))
	t.Cleanup(func() { assert.NoError(t, s.Stop()) })

	// wait til process starts up
	require.NoError(t, pingPong.AwaitPing())

	// save the pid
	process := s.GetProcess()

	// send pong to unblock the process so it can exit
	require.NoError(t, pingPong.SendPong())

	// wait til the respawned process pings again
	require.NoError(t, pingPong.AwaitPing())

	// test that a new process got respawned
	assert.NotEqual(t, process.Pid, s.GetProcess().Pid, "Respawn failed")
}

func TestStopWhileRespawn(t *testing.T) {
	fail := selectCmd(t,
		cmd{"false", []string{}},
		cmd{"sh", []string{"-c", "exit 1"}},
		cmd{"powershell", []string{"-noprofile", "-noninteractive", "-command", "exit 1"}},
	)

	s := Supervisor{
		Name:           "supervisor-test-stop-while-respawn",
		BinPath:        fail.binPath,
		Args:           fail.binArgs,
		RunDir:         t.TempDir(),
		TimeoutStop:    1 * time.Minute,
		TimeoutRespawn: 1 * time.Hour,
	}

	if assert.NoError(t, s.Supervise(t.Context()), "Failed to start") {
		// wait til the process exits
		for process := s.GetProcess(); ; {
			// Send "the null signal" to probe if the PID still exists
			// (https://www.man7.org/linux/man-pages/man3/kill.3p.html). On
			// Windows, the only emulated Signal is os.Kill, so this will return
			// EWINDOWS if the process is still running, i.e. the
			// WaitForSingleObject syscall on the process handle is still
			// blocking.
			err := process.Signal(syscall.Signal(0))

			// Wait a bit to ensure that the supervisor has noticed a potential
			// process exit as well, so that it's safe to assume that it reached
			// the respawn timeout internally.
			time.Sleep(100 * time.Millisecond)

			// Ensure that the error indicates that the process is done. Note
			// that on Windows, there seems to be a bug in os.Process that
			// causes EINVAL being returned instead of ErrProcessDone, probably
			// due to the wrong order of internal checks (i.e. the process
			// handle is checked before the done flag).
			if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.EINVAL) {
				break
			}
		}
	}

	// stop while waiting for respawn
	assert.NoError(t, s.Stop())
}

func TestMultiThread(t *testing.T) {
	sleep := selectCmd(t,
		cmd{"sleep", []string{"60"}},
		cmd{"powershell", []string{"-noprofile", "-noninteractive", "-command", "Start-Sleep -Seconds 60"}},
	)

	s := Supervisor{
		Name:    "supervisor-test-multithread",
		BinPath: sleep.binPath,
		Args:    sleep.binArgs,
		RunDir:  t.TempDir(),
	}

	var wg sync.WaitGroup
	assert.NoError(t, s.Supervise(t.Context()), "Failed to start")
	t.Cleanup(func() { assert.NoError(t, s.Stop()) })

	for range 255 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Stop()
			_ = s.Supervise(t.Context())
		}()
	}
	wg.Wait()
}

func TestCleanupPIDFile_Gracefully(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		t.Skip("PID file cleanup not implemented on macOS")
	}

	// Start some k0s-managed process.
	prevCmd, prevPingPong := pingpong.Start(t, pingpong.StartOptions{
		Env: []string{k0sManaged},
	})
	require.NoError(t, prevPingPong.AwaitPing())

	// Prepare another supervised process.
	pingPong := pingpong.New(t)
	s := Supervisor{
		Name:           t.Name(),
		BinPath:        pingPong.BinPath(),
		RunDir:         t.TempDir(),
		Args:           pingPong.BinArgs(),
		TimeoutStop:    1 * time.Minute,
		TimeoutRespawn: 1 * time.Hour,
	}

	// Create a PID file that's pointing to the running process.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, fmt.Appendf(nil, "%d\n", prevCmd.Process.Pid), 0644))

	// Start to supervise the new process.
	require.NoError(t, s.Supervise(t.Context()))
	t.Cleanup(func() {
		// Stop is called and checked in the regular test flow. This is just to
		// ensure it will be called in any case, so don't check the error here.
		_ = s.Stop()
	})

	// Expect the previous process to be gracefully terminated.
	assert.NoError(t, prevCmd.Wait())

	// Expect the new process to be started.
	assert.NoError(t, pingPong.AwaitPing())

	// Stop the supervisor and check if the PID file is gone.
	assert.NoError(t, s.Stop())
	assert.NoFileExists(t, pidFilePath)
}

func TestCleanupPIDFile_LingeringProcess(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		t.Skip("PID file cleanup not implemented on macOS")
	}

	// Start some k0s-managed process that won't terminate gracefully.
	prevCmd, prevPingPong := pingpong.Start(t, pingpong.StartOptions{
		Options: pingpong.Options{
			IgnoreGracefulTerminationRequests: true,
		},
		Env: []string{k0sManaged},
	})
	require.NoError(t, prevPingPong.AwaitPing())

	// Prepare another supervised process.
	pingPong := pingpong.New(t)
	s := Supervisor{
		Name:           t.Name(),
		BinPath:        pingPong.BinPath(),
		RunDir:         t.TempDir(),
		Args:           pingPong.BinArgs(),
		TimeoutStop:    10 * time.Millisecond,
		TimeoutRespawn: 1 * time.Hour,
	}

	// Create a PID file that's pointing to the running process.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, fmt.Appendf(nil, "%d\n", prevCmd.Process.Pid), 0644))

	// Start to supervise the new process and expect it to fail because the
	// previous process won't terminate.
	err := s.Supervise(t.Context())
	if !assert.Error(t, err) {
		assert.NoError(t, s.Stop())
	} else {
		assert.ErrorContains(t, err, "while waiting for termination of PID")
		assert.ErrorContains(t, err, pidFilePath)
		assert.ErrorContains(t, err, "process did not terminate in time")
	}

	// Expect the previous process to still be alive.
	require.NoError(t, prevPingPong.SendPong())

	// PID file should still point to the previous PID.
	if pid, err := os.ReadFile(pidFilePath); assert.NoError(t, err) {
		assert.Equal(t, fmt.Appendf(nil, "%d\n", prevCmd.Process.Pid), pid)
	}
}

func TestCleanupPIDFile_Cancel(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		t.Skip("PID file cleanup not yet implemented on Windows")
	case "darwin":
		t.Skip("PID file cleanup not implemented on macOS")
	}

	cmd, pingPong := pingpong.Start(t, pingpong.StartOptions{
		Options: pingpong.Options{IgnoreGracefulTerminationRequests: true},
		Env:     []string{k0sManaged},
	})

	s := Supervisor{
		Name:           t.Name(),
		BinPath:        pingPong.BinPath(),
		RunDir:         t.TempDir(),
		Args:           pingPong.BinArgs(),
		TimeoutStop:    1 * time.Minute,
		TimeoutRespawn: 1 * time.Hour,
	}

	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, fmt.Appendf(nil, "%d\n", cmd.Process.Pid), 0644))

	require.NoError(t, pingPong.AwaitPing())

	t.Run("context_timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(assert.AnError)
		err := s.Supervise(ctx)
		assert.ErrorContains(t, err, "while waiting for termination of PID")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("stop_timeout", func(t *testing.T) {
		s.TimeoutStop = 1 * time.Nanosecond
		err := s.Supervise(t.Context())
		assert.ErrorContains(t, err, "while waiting for termination of PID")
		assert.ErrorContains(t, err, "process did not terminate in time")
	})

	if pid, readErr := os.ReadFile(pidFilePath); assert.NoError(t, readErr) {
		assert.Equal(t, fmt.Appendf(nil, "%d\n", cmd.Process.Pid), pid)
	}
}

func TestCleanupPIDFile_WrongProcess(t *testing.T) {
	// Start some process that's not managed by k0s.
	prevCmd, prevPingPong := pingpong.Start(t, pingpong.StartOptions{})
	require.NoError(t, prevPingPong.AwaitPing())

	// Prepare another supervised process.
	pingPong := pingpong.New(t)
	s := Supervisor{
		Name:           t.Name(),
		BinPath:        pingPong.BinPath(),
		RunDir:         t.TempDir(),
		Args:           pingPong.BinArgs(),
		TimeoutStop:    1 * time.Minute,
		TimeoutRespawn: 1 * time.Hour,
	}

	// Create a PID file that's pointing to the running process.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, fmt.Appendf(nil, "%d\n", prevCmd.Process.Pid), 0644))

	// Start to supervise the new process.
	require.NoError(t, s.Supervise(t.Context()))
	t.Cleanup(func() { assert.NoError(t, s.Stop()) })

	// Expect the PID file to be replaced with the new PID.
	if pid, err := os.ReadFile(pidFilePath); assert.NoError(t, err, "Failed to read PID file") {
		assert.Equal(t, fmt.Appendf(nil, "%d\n", s.cmd.Process.Pid), pid)
	}

	// Expect the previous process to be still alive and react to the pong signal.
	if assert.NoError(t, prevPingPong.SendPong()) {
		assert.NoError(t, prevCmd.Wait())
	}
}

func TestCleanupPIDFile_NonexistingProcess(t *testing.T) {
	// Prepare some supervised process.
	pingPong := pingpong.New(t)
	s := Supervisor{
		Name:    t.Name(),
		BinPath: pingPong.BinPath(),
		RunDir:  t.TempDir(),
		Args:    pingPong.BinArgs(),
	}

	// Create a PID file that's pointing to some non-existing process. Note that
	// this is probably not 100% safe, but we'll assume MaxInt32 will be unused.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, fmt.Appendf(nil, "%d\n", math.MaxInt32), 0644))

	// Start to supervise the new process.
	require.NoError(t, s.Supervise(t.Context()))
	t.Cleanup(func() { assert.NoError(t, s.Stop()) })

	// Expect the PID file to be replaced with the new PID.
	if pid, err := os.ReadFile(pidFilePath); assert.NoError(t, err, "Failed to read PID file") {
		assert.Equal(t, fmt.Appendf(nil, "%d\n", s.cmd.Process.Pid), pid)
	}
}

func TestCleanupPIDFile_BogusPIDFile(t *testing.T) {
	// Prepare some supervised process that should never be started.
	s := Supervisor{
		Name:    t.Name(),
		BinPath: filepath.Join(t.TempDir(), "foo"),
		RunDir:  t.TempDir(),
	}

	// Create a PID file with non-parsable content.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, []byte("rubbish"), 0644))

	// Expect the supervisor to bail out.
	assert.ErrorContains(t, s.Supervise(t.Context()), `"rubbish": invalid`)
}

type cmd struct {
	binPath string
	binArgs []string
}

func selectCmd(t *testing.T, cmds ...cmd) (_ cmd) {
	var tested []string
	for _, candidate := range cmds {
		if path, err := exec.LookPath(candidate.binPath); err == nil {
			return cmd{path, candidate.binArgs}
		}
		tested = append(tested, candidate.binPath)
	}

	require.Failf(t, "none of those executables in PATH, dunno how to create test process: %s", strings.Join(tested, ", "))
	return // diverges above
}

func TestMain(m *testing.M) {
	pingpong.Hook()
	TerminationHelperHook()
	os.Exit(m.Run())
}
