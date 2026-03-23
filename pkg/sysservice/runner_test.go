// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestExecRunnerRun_ReturnsExitCodeAndCombinedOutputWithoutError(t *testing.T) {
	runner := ExecRunner{}
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	exit, stdout, stderr, err := runner.Run(
		context.Background(),
		os.Args[0],
		"-test.run=TestExecRunnerHelperProcess",
		"--",
		"stdout-line",
		"stderr-line",
		"7",
	)

	if err != nil {
		t.Fatalf("expected nil error for non-zero exit, got %v", err)
	}
	if exit != 7 {
		t.Fatalf("expected exit code 7, got %d", exit)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "stdout-line") {
		t.Fatalf("expected stdout to contain stdout output, got %q", stdout)
	}
	if !strings.Contains(stdout, "stderr-line") {
		t.Fatalf("expected stdout to contain stderr output from CombinedOutput, got %q", stdout)
	}
}

func TestExecRunnerRun_ReturnsExitCodeOneWhenCommandCannotStart(t *testing.T) {
	runner := ExecRunner{}

	exit, stdout, stderr, err := runner.Run(context.Background(), "definitely-not-a-real-command-k0s-test")

	if err == nil {
		t.Fatal("expected command error")
	}
	if exit != 1 {
		t.Fatalf("expected exit code 1, got %d", exit)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
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
