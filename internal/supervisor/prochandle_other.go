//go:build !linux && !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"runtime"
)

func (s *Supervisor) cleanupPID(_ context.Context, pid int) error {
	return fmt.Errorf("%w on %s: cleanup for PID %d from PID file %s", errors.ErrUnsupported, runtime.GOOS, pid, s.PidFile)
}
