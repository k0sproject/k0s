//go:build windows

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package windows_test

import (
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/k0sproject/k0s/internal/os/windows"
	"github.com/k0sproject/k0s/internal/testutil/pingpong"

	syswindows "golang.org/x/sys/windows"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcHandle_Close(t *testing.T) {
	cmd, pingPong := pingpong.Start(t)
	require.NoError(t, pingPong.AwaitPing())

	underTest, err := windows.OpenProcess(uint32(cmd.Process.Pid))
	require.NoError(t, err)
	assert.NoError(t, underTest.Close())
	assert.ErrorIs(t, underTest.Close(), syswindows.ERROR_INVALID_HANDLE)
}

func TestProcHandle_NoSuchProcess(t *testing.T) {
	// We blindly assume that MaxUint32 is unused. YOLO!
	handle, err := windows.OpenProcess(math.MaxUint32)
	if err == nil {
		assert.NoError(t, handle.Close())
	}
	assert.ErrorIs(t, err, syscall.ESRCH)
}

func TestProcHandle_IsTerminated(t *testing.T) {
	cmd, pingPong := pingpong.Start(t)
	require.NoError(t, pingPong.AwaitPing())

	underTest, err := windows.OpenProcess(uint32(cmd.Process.Pid))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, underTest.Close()) })

	var checked atomic.Bool
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			exited, err := underTest.Exited()
			checked.Store(true)
			if !assert.NoError(t, err) || !exited {
				return
			}
		}
	}()
	t.Cleanup(func() { <-done })

	// Wait for the terminate check to happen once.
	for !checked.Load() {
		runtime.Gosched()
	}

	require.NoError(t, pingPong.SendPong())
}

func TestProcHandle_Environ(t *testing.T) {
	envVar := "__PROCHANDLE_TEST=" + t.Name()
	cmd, pingPong := pingpong.Start(t, pingpong.StartOptions{
		Env: []string{envVar},
	})
	require.NoError(t, pingPong.AwaitPing())

	underTest, err := windows.OpenProcess(uint32(cmd.Process.Pid))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, underTest.Close()) })

	// Set up a way to close the process concurrently, so the code is tested in
	// a multi-threaded way. This is mainly to potentially trigger the partial
	// copy error in ReadProcessMemory. In practice, however, it is highly
	// improbable that this code path will be hit, since the syscalls are too
	// fast to accidentally interleave with the process termination.
	exited := make(chan struct{})
	exit := make(chan struct{})
	signalExit := sync.OnceFunc(func() { close(exit) })
	go func() {
		defer close(exited)
		<-exit
		runtime.Gosched()
		if assert.NoError(t, pingPong.SendPong()) {
			runtime.Gosched()
			assert.NoError(t, cmd.Wait())
		}
	}()
	t.Cleanup(func() { signalExit(); <-exited })

	env, err := underTest.Environ()
	require.NoError(t, err)
	assert.Contains(t, env, envVar)

	for {
		env, err := underTest.Environ()
		if err == nil {
			assert.Contains(t, env, envVar)
			signalExit()
		} else {
			require.ErrorIs(t, err, os.ErrProcessDone)
			return
		}
	}

}

func TestMain(m *testing.M) {
	pingpong.Hook()
	os.Exit(m.Run())
}
