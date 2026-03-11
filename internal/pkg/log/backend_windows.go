// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	internalio "github.com/k0sproject/k0s/internal/io"
	oswindows "github.com/k0sproject/k0s/internal/os/windows"

	"k8s.io/utils/ptr"

	"github.com/sirupsen/logrus"
	syswindows "golang.org/x/sys/windows"
)

type LogFileBackend interface {
	Backend
	InitLogFile(func() (*os.File, error)) error
}

func installBackend() (Backend, ShutdownLoggingFunc) {
	if isService, err := oswindows.IsService(); err != nil {
		panic(err)
	} else if isService {
		// When running as a service, both stdout and stderr go into Nirvana.
		// Hence, k0s needs to write its log output to log files.
		backend, closeBackend := installLogFileBackend()

		// Bridge the stdout/stderr file descriptors to the logger,
		// just in case that anything else tries to use them.
		closePipes := installStdoutStderrPipes()

		return backend, func() {
			// Properly flush/shutdown the pipes before the log file backend
			closePipes()
			closeBackend()
		}
	}

	return nil, func() {}
}

// Installs a backend that accumulates log data until a log file is available.
func installLogFileBackend() (*logFileBackend, func()) {
	backend, buffered, closeBackend := newLogFileBackend()
	logrus.SetOutput(buffered)
	return backend, closeBackend
}

type logFileBackend struct {
	state atomic.Pointer[any]
}

var _ LogFileBackend = (*logFileBackend)(nil)

type logBuffer = struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	file *os.File
}

func newLogFileBackend() (*logFileBackend, io.Writer, func()) {
	buf, backend := new(logBuffer), new(logFileBackend)
	backend.state.Store(ptr.To[any](buf))

	writer := internalio.WriterFunc(func(b []byte) (int, error) {
		buf.mu.Lock()
		defer buf.mu.Unlock()
		if buf.file != nil {
			return buf.file.Write(b)
		}
		return buf.buf.Write(b)
	})

	close := func() {
		if state := backend.state.Swap(nil); state != nil {
			if closer, ok := (*state).(io.Closer); ok {
				logrus.Info("Closing logging backend, good bye ...")
				_ = closer.Close()
			}
		}
	}

	return backend, writer, close
}

func (b *logFileBackend) InitLogFile(openFile func() (*os.File, error)) error {
	var state any
	if statePtr := b.state.Load(); statePtr != nil {
		state = *statePtr
	}

	switch state := state.(type) {
	case *os.File:
		return errors.New("log file has already been set")

	case *logBuffer:
		if file, err := drainBuffer(state, openFile); err != nil {
			return err
		} else {
			logrus.SetOutput(file)           // Set the new output file to logrus.
			b.state.Store(ptr.To[any](file)) // Store it as the new active state.
			return nil
		}

	default:
		return fmt.Errorf("%w: not initialized", errors.ErrUnsupported)
	}
}

func drainBuffer(b *logBuffer, openFile func() (*os.File, error)) (*os.File, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.file != nil {
		return nil, errors.New("log file has already been set")
	}

	// Open a new log file and write the buffer into it.
	file, err := openFile()
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(b.buf.Bytes()); err != nil {
		return nil, err
	}
	// Store the file in the buffer, so subsequent writes won't be buffered.
	b.file = file

	return file, nil
}

func installStdoutStderrPipes() (close func()) {
	var pipes sync.WaitGroup
	stdout, stderr := pipeToLogger(&pipes, "stdout"), pipeToLogger(&pipes, "stderr")
	_, _ = os.Stdout.Close(), os.Stderr.Close()
	_ = syswindows.SetStdHandle(syswindows.STD_OUTPUT_HANDLE, syswindows.Handle(stdout.Fd()))
	_ = syswindows.SetStdHandle(syswindows.STD_ERROR_HANDLE, syswindows.Handle(stderr.Fd()))
	os.Stdout, os.Stderr = stdout, stderr
	return func() { _, _ = stdout.Close(), stderr.Close(); pipes.Wait() }
}

func pipeToLogger(wg *sync.WaitGroup, stream string) *os.File {
	pipe, src, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	dst := NewWriter(logrus.WithFields(logrus.Fields{"component": "k0s", "stream": stream}), logrus.InfoLevel, 16*1024)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pipe.Close()
		_, _ = pipe.WriteTo(dst)
	}()

	return src
}
