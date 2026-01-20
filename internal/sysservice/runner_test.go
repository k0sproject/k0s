// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const exitHelperMarker = "__SYSSERVICE_EXIT_HELPER"

func TestMain(m *testing.M) {
	// React to command lines like "<exe> <marker> <stderr text> <exit code>"
	if len(os.Args) == 4 && os.Args[1] == exitHelperMarker {
		fmt.Fprintln(os.Stderr, os.Args[2])
		code, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: invalid exit code:", err)
			os.Exit(1)
		}
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestExecRunnerRun_ReturnsExitError(t *testing.T) {
	err := execRunner{}.Run(t.Context(), os.Args[0], exitHelperMarker, "stderr-line", "7")

	assert.EqualError(t, err, "exit status 7: stderr-line")
	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	assert.Equal(t, 7, ee.ExitCode())
}

func TestExecRunnerRun_ErrorWhenCommandNotFound(t *testing.T) {
	runner := execRunner{}
	require.Error(t, runner.Run(t.Context(), "definitely-not-a-real-command-k0s-test"))
}
