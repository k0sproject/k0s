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

var backend atomic.Pointer[any]

type logBuffer = struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	file *os.File
}

func initBackend() ShutdownLoggingFunc {
	if isService, err := oswindows.IsService(); err != nil {
		panic(err)
	} else if !isService {
		return func() {}
	}

	// Install a backend that accumulates log data until a log file is available.
	var buf logBuffer
	logrus.SetOutput(internalio.WriterFunc(func(p []byte) (int, error) {
		buf.mu.Lock()
		defer buf.mu.Unlock()
		if buf.file != nil {
			return buf.file.Write(p)
		}
		return buf.buf.Write(p)
	}))
	backend.Store(ptr.To[any](&buf))

	// When running as a service, both stdout and stderr go into nirvana.
	// Bridge those to the logger.
	var pipes sync.WaitGroup
	stdout, stderr := pipeToLogger(&pipes, "stdout"), pipeToLogger(&pipes, "stderr")
	_, _ = os.Stdout.Close(), os.Stderr.Close()
	_ = syswindows.SetStdHandle(syswindows.STD_OUTPUT_HANDLE, syswindows.Handle(stdout.Fd()))
	_ = syswindows.SetStdHandle(syswindows.STD_ERROR_HANDLE, syswindows.Handle(stderr.Fd()))
	os.Stdout, os.Stderr = stdout, stderr

	return func() {
		_ = stdout.Close()
		_ = stderr.Close()
		pipes.Wait()
		if backend := backend.Load(); backend != nil {
			if closer, ok := (*backend).(io.Closer); ok {
				_ = closer.Close()
			}
		}
	}
}

func InitLogFile(openFile func() (*os.File, error)) error {
	var b any
	if bPtr := backend.Load(); bPtr != nil {
		b = *bPtr
	}

	switch b := b.(type) {
	case *os.File:
		return errors.New("log file has already been set")
	case *logBuffer:
		return drainBuffer(b, openFile)
	default:
		return fmt.Errorf("%w: not initialized", errors.ErrUnsupported)
	}
}

func drainBuffer(b *logBuffer, openFile func() (*os.File, error)) error {
	mu := &b.mu
	mu.Lock()
	defer func() {
		if mu != nil {
			mu.Unlock()
		}
	}()

	if b.file != nil {
		return errors.New("log file has already been set")
	}

	// Open a new log file and write the buffer into it.
	file, err := openFile()
	if err != nil {
		return err
	}
	if _, err := file.Write(b.buf.Bytes()); err != nil {
		return err
	}
	// Store the file in the buffer, so subsequent writes won't be buffered.
	b.file = file

	mu.Unlock()
	mu = nil

	// Set the new output file to logrus.
	logrus.SetOutput(file)
	// Store it as the new active backend.
	backend.Store(ptr.To[any](file))

	return nil
}

func pipeToLogger(wg *sync.WaitGroup, stream string) *os.File {
	pipe, src, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	dst := NewWriter(logrus.WithFields(logrus.Fields{"component": "k0s", "stream": stream}), 16*1024)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pipe.Close()
		_, _ = pipe.WriteTo(dst)
	}()

	return src
}
