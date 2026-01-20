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
	exit   int
	stdout string
	stderr string
	err    error
}

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

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) (int, string, string, error) {
	r.calls = append(r.calls, call{name: name, args: append([]string{}, args...)})
	k := r.key(name, args...)
	if rep, ok := r.replies[k]; ok {
		return rep.exit, rep.stdout, rep.stderr, rep.err
	}
	return 1, "", "", fmt.Errorf("unexpected command: %s", k)
}
