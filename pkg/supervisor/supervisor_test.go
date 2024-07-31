/*
Copyright 2021 k0s authors

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
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

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
		{
			expectedErrMsg: "mkdir " + sleep.binPath,
			proc: Supervisor{
				Name:    "supervisor-test-rundir-init-fail",
				BinPath: sleep.binPath,
				Args:    sleep.binArgs,
				RunDir:  filepath.Join(sleep.binPath, "obstructed"),
			},
		},
	}

	for _, s := range testSupervisors {
		t.Run(s.proc.Name, func(t *testing.T) {
			err := s.proc.Supervise()
			if s.expectedErrMsg != "" {
				assert.ErrorContains(t, err, s.expectedErrMsg)
			} else {
				assert.NoError(t, err, "Failed to start")
			}
			s.proc.Stop()
		})
	}
}

func TestGetEnv(t *testing.T) {
	// backup environment vars, and restore them when test finishes
	oldEnv := os.Environ()
	t.Cleanup(func() {
		for _, e := range oldEnv {
			key, val, _ := strings.Cut(e, "=")
			assert.NoError(t, os.Setenv(key, val))
		}
	})

	os.Clearenv()
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

	env := getEnv("/var/lib/k0s", "foo", false)
	sort.Strings(env)
	expected := fmt.Sprintf("[HTTPS_PROXY=a.b.c:1080 PATH=/var/lib/k0s/bin%c/usr/local/bin _K0S_MANAGED=yes k1=v1 k2=foo_v2 k3=foo_v3 k4=v4]", os.PathListSeparator)
	actual := fmt.Sprintf("%s", env)
	assert.Equal(t, expected, actual)

	env = getEnv("/var/lib/k0s", "foo", true)
	sort.Strings(env)
	expected = fmt.Sprintf("[FOO_PATH=/usr/local/bin FOO_k2=foo_v2 FOO_k3=foo_v3 HTTPS_PROXY=a.b.c:1080 PATH=/var/lib/k0s/bin%c/bin _K0S_MANAGED=yes k1=v1 k2=v2 k3=v3 k4=v4]", os.PathListSeparator)
	actual = fmt.Sprintf("%s", env)
	assert.Equal(t, expected, actual)
}

func TestRespawn(t *testing.T) {
	pingPong := pingpong.New(t)

	s := Supervisor{
		Name:           t.Name(),
		BinPath:        pingPong.BinPath(),
		RunDir:         t.TempDir(),
		Args:           pingPong.BinArgs(),
		TimeoutRespawn: 1 * time.Millisecond,
	}
	require.NoError(t, s.Supervise())
	t.Cleanup(s.Stop)

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
		TimeoutRespawn: 1 * time.Hour,
	}

	if assert.NoError(t, s.Supervise(), "Failed to start") {
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
	s.Stop()
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
	assert.NoError(t, s.Supervise(), "Failed to start")
	t.Cleanup(s.Stop)

	for i := 0; i < 255; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Stop()
			_ = s.Supervise()
		}()
	}
	wg.Wait()
}

func TestCleanupPIDFile_Gracefully(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PID file cleanup not yet implemented on Windows")
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
		TimeoutStop:    1 * time.Second,
		TimeoutRespawn: 1 * time.Hour,
	}

	// Create a PID file that's pointing to the running process.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", prevCmd.Process.Pid)), 0644))

	// Start to supervise the new process.
	require.NoError(t, s.Supervise())
	t.Cleanup(s.Stop)

	// Expect the previous process to be gracefully terminated.
	assert.NoError(t, prevCmd.Wait())

	// Stop the supervisor and check if the PID file is gone.
	s.Stop()
	assert.NoFileExists(t, pidFilePath)
}

func TestCleanupPIDFile_Forcefully(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PID file cleanup not yet implemented on Windows")
	}

	// Start some k0s-managed process that won't terminate gracefully.
	prevCmd, prevPingPong := pingpong.Start(t, pingpong.StartOptions{
		Env:                           []string{k0sManaged},
		IgnoreGracefulShutdownRequest: true,
	})
	require.NoError(t, prevPingPong.AwaitPing())

	// Prepare another supervised process.
	pingPong := pingpong.New(t)
	s := Supervisor{
		Name:           t.Name(),
		BinPath:        pingPong.BinPath(),
		RunDir:         t.TempDir(),
		Args:           pingPong.BinArgs(),
		TimeoutStop:    1 * time.Second,
		TimeoutRespawn: 1 * time.Second,
	}

	// Create a PID file that's pointing to the running process.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", prevCmd.Process.Pid)), 0644))

	// Start to supervise the new process.
	require.NoError(t, s.Supervise())
	t.Cleanup(s.Stop)

	// Expect the previous process to be forcefully terminated.
	assert.ErrorContains(t, prevCmd.Wait(), "signal: killed")

	// Stop the supervisor and check if the PID file is gone.
	assert.NoError(t, pingPong.AwaitPing())
	s.Stop()
	assert.NoFileExists(t, pidFilePath)
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
		TimeoutStop:    1 * time.Second,
		TimeoutRespawn: 1 * time.Second,
	}

	// Create a PID file that's pointing to the running process.
	pidFilePath := filepath.Join(s.RunDir, s.Name+".pid")
	require.NoError(t, os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", prevCmd.Process.Pid)), 0644))

	// Start to supervise the new process.
	require.NoError(t, s.Supervise())
	t.Cleanup(s.Stop)

	// Expect the PID file to be replaced with the new PID.
	if pid, err := os.ReadFile(pidFilePath); assert.NoError(t, err, "Failed to read PID file") {
		assert.Equal(t, []byte(fmt.Sprintf("%d\n", s.cmd.Process.Pid)), pid)
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
	require.NoError(t, os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", math.MaxInt32)), 0644))

	// Start to supervise the new process.
	require.NoError(t, s.Supervise())
	t.Cleanup(s.Stop)

	// Expect the PID file to be replaced with the new PID.
	if pid, err := os.ReadFile(pidFilePath); assert.NoError(t, err, "Failed to read PID file") {
		assert.Equal(t, []byte(fmt.Sprintf("%d\n", s.cmd.Process.Pid)), pid)
	}
}

func TestCleanupPIDFile_BogusPIDFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PID file cleanup not yet implemented on Windows")
	}

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
	assert.ErrorContains(t, s.Supervise(), `"rubbish": invalid`)
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

	require.Fail(t, "none of those executables in PATH, dunno how to create test process: %s", strings.Join(tested, ", "))
	return // diverges above
}
