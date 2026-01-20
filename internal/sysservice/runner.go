// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	_, err := exec.CommandContext(ctx, name, args...).Output()
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}

// Returns the exit code that err represents, or -1 if err doesn't carry any
// exit code information.
func exitCode(err error) int {
	// Not using exec.ExitError directly as it's not possible to create errors
	// with arbitrary exit codes for testing.
	type exitError interface {
		error
		ExitCode() int
	}

	if exitErr, ok := errors.AsType[exitError](err); ok {
		return exitErr.ExitCode()
	}
	return -1
}
