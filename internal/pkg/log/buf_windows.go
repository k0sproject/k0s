// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/k0sproject/k0s/internal/supervised"
	"github.com/sirupsen/logrus"
)

type logBuffer struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	file *os.File
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if logFile != nil {
		return logFile.Write(p)
	}
	return b.buf.Write(p)
}

var (
	logBuf  *logBuffer
	logFile *os.File
)

func initBuffer() {
	if isService, err := supervised.IsService(); err != nil {
		panic(err)
	} else if !isService {
		return
	}

	buf := new(logBuffer)
	logBuf = buf
	logrus.SetOutput(buf)
}

func SwapBufferedOutput(openFile func() (*os.File, error)) error {
	logBuf := logBuf
	if logBuf == nil {
		return fmt.Errorf("%w: no buffer to swap", errors.ErrUnsupported)
	}

	mu := &logBuf.mu
	mu.Lock()
	defer func() {
		if mu != nil {
			mu.Unlock()
		}
	}()

	if logFile != nil {
		return fmt.Errorf("%w: already swapped", errors.ErrUnsupported)
	}
	file, err := openFile()
	if err != nil {
		return err
	}
	if _, err := file.Write(logBuf.buf.Bytes()); err != nil {
		return err
	}
	logFile = file

	mu.Unlock()
	mu = nil

	logrus.SetOutput(file)
	logBuf = nil

	return nil
}

func shutdownBuffer() {
	_ = logFile.Close()
}
