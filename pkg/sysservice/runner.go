// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (exitCode int, stdout string, stderr string, err error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (int, string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()

	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		} else {
			exit = 1
		}
	}
	return exit, string(out), "", err
}
