//go:build !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"errors"
	"os"
)

func SwapBufferedOutput(func() (*os.File, error)) error { return errors.ErrUnsupported }

func initBuffer()     {}
func shutdownBuffer() {}
