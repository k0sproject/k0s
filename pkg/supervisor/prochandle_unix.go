//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"fmt"
	"os"
	"syscall"
)

func TerminationHelperHook() {
	// no-op
}

func requestGracefulTermination(p *os.Process) error {
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	return nil
}
