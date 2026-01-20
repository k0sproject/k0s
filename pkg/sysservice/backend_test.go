// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"fmt"
	"strings"
)

type call struct {
	name string
	args []string
}

type reply struct {
	exit int
	err  error
}

// fakeExitError implements exitCoder so exitCode() works in tests without
// needing a real subprocess.
type fakeExitError struct{ code int }

func (e *fakeExitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }
func (e *fakeExitError) ExitCode() int { return e.code }

type fakeRunner struct {
	calls   []call
	replies map[string]reply
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{replies: map[string]reply{}}
}

func (r *fakeRunner) key(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func (r *fakeRunner) When(name string, args []string, rep reply) {
	r.replies[r.key(name, args...)] = rep
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	r.calls = append(r.calls, call{name: name, args: append([]string{}, args...)})
	k := r.key(name, args...)
	if rep, ok := r.replies[k]; ok {
		if rep.err != nil {
			return rep.err
		}
		if rep.exit != 0 {
			return &fakeExitError{rep.exit}
		}
		return nil
	}
	return fmt.Errorf("unexpected command: %s", k)
}
