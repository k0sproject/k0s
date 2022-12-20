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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createMockProcess(procFSPath string, pid string, cmdline []byte, k0smanaged bool) error {
	if err := os.Mkdir(filepath.Join(procFSPath, pid), 0700); err != nil {
		return fmt.Errorf("Failed to create fake proc dir: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(procFSPath, pid, "cmdline"), cmdline, 0700); err != nil {
		return fmt.Errorf("Failed to write cmdline: %v", err)
	}

	var env []byte
	if k0smanaged {
		env = []byte(fmt.Sprintf("ENV1=A\x00%s\x00ENV3=B\x00", k0sManaged))
	} else {
		env = []byte("ENV1=A\x00ENV3=B\x00")
	}
	if err := ioutil.WriteFile(filepath.Join(procFSPath, pid, "environ"), env, 0700); err != nil {
		return fmt.Errorf("Failed to write environ: %v", err)
	}
	return nil
}

type mockKiller struct {
	ProcFSPath string
}

func (m *mockKiller) killNow(pid int, s syscall.Signal) error {
	// os.RemoveAll() doesn't return an error if the path doesn't exist
	// so we need to check that first.
	path := filepath.Join(m.ProcFSPath, strconv.Itoa(pid))
	if _, err := os.Stat(path); err != nil {
		return syscall.ESRCH
	}

	err := os.RemoveAll(path)
	return err
}

func (m *mockKiller) killOnSigKill(pid int, s syscall.Signal) error {
	// os.RemoveAll() doesn't return an error if the path doesn't exist
	// so we need to check that first.
	path := filepath.Join(m.ProcFSPath, strconv.Itoa(pid))
	if _, err := os.Stat(path); err != nil {
		return syscall.ESRCH
	}

	if s == syscall.SIGKILL {
		return os.RemoveAll(path)
	}
	return nil
}

func TestKillPid(t *testing.T) {
	t.Run("Kill immediately", func(t *testing.T) {
		m := mockKiller{
			ProcFSPath: t.TempDir(),
		}
		s := Supervisor{
			Name:         "supervisor-test-is-running",
			ProcFSPath:   m.ProcFSPath,
			KillFunction: m.killNow,
			BinPath:      "/bin/true",
		}
		require.NoError(t, createMockProcess(s.ProcFSPath, "123", []byte("/bin/true\x00"), true))

		check := make(chan time.Time)
		deadline := make(chan time.Time)
		defer close(check)
		defer close(deadline)

		go func() {
			check <- time.Now()
			check <- time.Now()
			deadline <- time.Now()
		}()
		require.NoError(t, s.killPid(123, check, deadline), "Failed to kill pid")

		// If we can read deadline, then we guarantee that check was read twice because it's
		// unbuffered. This guarantees the loop has run exactly twice.
		<-deadline
	})

	t.Run("Force SigKill", func(t *testing.T) {
		m := mockKiller{
			ProcFSPath: t.TempDir(),
		}
		s := Supervisor{
			Name:         "supervisor-test-is-running",
			TimeoutStop:  exitCheckInterval,
			ProcFSPath:   m.ProcFSPath,
			KillFunction: m.killOnSigKill,
			BinPath:      "/bin/true",
		}
		require.NoError(t, createMockProcess(s.ProcFSPath, "123", []byte("/bin/true\x00"), true))

		check := make(chan time.Time)
		deadline := make(chan time.Time)
		verify := make(chan time.Time)
		defer close(check)
		defer close(deadline)
		defer close(verify)

		go func() {
			check <- time.Now()
			deadline <- time.Now()
			verify <- time.Now()
		}()
		require.NoError(t, s.killPid(123, check, deadline), "Error deleting fake pid")

		<-verify
	})
}

func TestMaybeKillPidFile(t *testing.T) {
	t.Run("Kill non existing pidFile", func(t *testing.T) {
		s := Supervisor{
			PidFile: filepath.Join(t.TempDir(), "invalid"),
		}
		require.NoError(t, s.maybeKillPidFile(nil, nil), "If the file doesn't exist should exit without error")
	})

	t.Run("Kill invalid pidFile", func(t *testing.T) {
		s := Supervisor{
			PidFile: filepath.Join(t.TempDir(), "invalid"),
		}
		require.NoError(t, ioutil.WriteFile(s.PidFile, []byte("invalid"), 0600), "Failed to create invalid pidFile")

		require.Error(t, s.maybeKillPidFile(nil, nil), "Should have failed to kill invalid pidFile")
	})
	t.Run("Don't kill validPidFile pointing to different process", func(t *testing.T) {
		m := mockKiller{
			ProcFSPath: t.TempDir(),
		}
		s := Supervisor{
			PidFile:      filepath.Join(t.TempDir(), "valid"),
			ProcFSPath:   m.ProcFSPath,
			KillFunction: m.killNow,
			BinPath:      "/bin/true",
		}

		require.NoError(t, createMockProcess(s.ProcFSPath, "12345", []byte("/bin/false\x00"), true))
		require.NoError(t, ioutil.WriteFile(s.PidFile, []byte("12345\n"), 0700), "Failed to create valid pidFile")

		check := make(chan time.Time)
		deadline := make(chan time.Time)
		defer close(check)
		defer close(deadline)
		go func() {
			check <- time.Now()
		}()

		require.NoError(t, s.maybeKillPidFile(check, deadline), "Error killing valid pidFile")

		_, err := os.Stat(filepath.Join(m.ProcFSPath, "12345"))
		require.NoError(t, err, "Should not have killed the process")
	})

	t.Run("Don't kill process without the env", func(t *testing.T) {
		m := mockKiller{
			ProcFSPath: t.TempDir(),
		}
		s := Supervisor{
			PidFile:      filepath.Join(t.TempDir(), "valid"),
			ProcFSPath:   m.ProcFSPath,
			KillFunction: m.killNow,
			BinPath:      "/bin/true",
		}

		require.NoError(t, createMockProcess(s.ProcFSPath, "12345", []byte("/bin/true\x00arg1\x00arg2\x00"), false))
		require.NoError(t, ioutil.WriteFile(s.PidFile, []byte("12345\n"), 0700), "Failed to create valid pidFile")

		check := make(chan time.Time)
		deadline := make(chan time.Time)
		defer close(check)
		defer close(deadline)
		go func() {
			check <- time.Now()
		}()
		require.NoError(t, s.maybeKillPidFile(check, deadline), "Error killing valid pidFile")

		_, err := os.Stat(filepath.Join(m.ProcFSPath, "12345"))
		require.NoError(t, err, "Should not have killed the process")
	})

	t.Run("Kill process which must be killed", func(t *testing.T) {
		m := mockKiller{
			ProcFSPath: t.TempDir(),
		}
		s := Supervisor{
			PidFile:      filepath.Join(t.TempDir(), "valid"),
			ProcFSPath:   m.ProcFSPath,
			KillFunction: m.killNow,
			BinPath:      "/bin/true",
		}

		require.NoError(t, createMockProcess(s.ProcFSPath, "12345", []byte("/bin/true\x00arg1\x00arg2\x00"), true))
		require.NoError(t, ioutil.WriteFile(s.PidFile, []byte("12345\n"), 0700), "Failed to create valid pidFile")

		check := make(chan time.Time)
		deadline := make(chan time.Time)
		defer close(check)
		defer close(deadline)

		go func() {
			check <- time.Now()
			check <- time.Now()
		}()

		require.NoError(t, s.maybeKillPidFile(check, deadline), "Error killing valid pidFile")

		_, err := os.Stat(filepath.Join(m.ProcFSPath, "12345"))
		require.Error(t, err, "Should have killed the process")
	})
}
