// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecRunnerRun_ReturnsExitError(t *testing.T) {
	runner := execRunner{}
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	err := runner.Run(
		t.Context(),
		os.Args[0],
		"-test.run=TestExecRunnerHelperProcess",
		"--",
		"stdout-line",
		"stderr-line",
		"7",
	)

	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	assert.Equal(t, 7, ee.ExitCode())
	assert.Equal(t, "stderr-line\n", string(ee.Stderr))
}

func TestExecRunnerRun_ErrorWhenCommandNotFound(t *testing.T) {
	runner := execRunner{}
	require.Error(t, runner.Run(t.Context(), "definitely-not-a-real-command-k0s-test"))
}

func TestExecRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for i, arg := range args {
		if arg != "--" {
			continue
		}
		args = args[i+1:]
		break
	}

	if len(args) != 3 {
		fmt.Fprintf(os.Stderr, "unexpected helper args: %v", args)
		os.Exit(2)
	}

	fmt.Fprintln(os.Stdout, args[0])
	fmt.Fprintln(os.Stderr, args[1])

	var exitCode int
	if _, err := fmt.Sscanf(args[2], "%d", &exitCode); err != nil {
		fmt.Fprintf(os.Stderr, "invalid exit code: %v", err)
		os.Exit(2)
	}
	os.Exit(exitCode)
}
