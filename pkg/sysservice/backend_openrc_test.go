// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRCInstall_WritesInitScript_UnderRoot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))
	r := newFakeRunner()

	svc := newOpenRC("k0scontroller")
	svc.root = root
	svc.runner = r
	svc.exec = func() (string, error) { return "/usr/local/bin/k0s", nil }

	require.NoError(t, svc.Install(ctx, []string{"controller", "--config", "/etc/k0s/k0s.yaml"}, []string{"K0S_FOO=bar", "K0S_BAR=baz"}))

	initPath := filepath.Join(root, "etc/init.d/k0scontroller")
	initBytes, err := os.ReadFile(initPath)
	require.NoError(t, err, "expected init script at %s", initPath)

	initTxt := string(initBytes)
	assert.Contains(t, initTxt, `command="/usr/local/bin/k0s"`)
	assert.Contains(t, initTxt, `command_args="'controller' '--config' '/etc/k0s/k0s.yaml' "`)
	assert.Contains(t, initTxt, `supervise-daemon`)
	assert.Contains(t, initTxt, "export K0S_FOO='bar'")
	assert.Contains(t, initTxt, "export K0S_BAR='baz'")
	assert.Empty(t, r.calls, "expected no service manager calls during install")
}

func TestOpenRCInstall_FailsIfAlreadyInstalled(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))
	r := newFakeRunner()

	svc := newOpenRC("k0scontroller")
	svc.root = root
	svc.runner = r
	svc.exec = func() (string, error) { return "/usr/local/bin/k0s", nil }

	require.NoError(t, svc.Install(ctx, nil, nil), "first Install() should succeed")

	assert.ErrorIs(t, svc.Install(ctx, nil, nil), fs.ErrExist)
}

func TestQuoteOpenRCEnv(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`KEY=value`, `KEY='value'`},
		{`KEY=has space`, `KEY='has space'`},
		{`KEY=$dollar`, `KEY='$dollar'`},
		{`KEY=$(subshell)`, `KEY='$(subshell)'`},
		{`KEY=it's`, `KEY='it'\''s'`},
		{`NOEQUAL`, `NOEQUAL`},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, quoteOpenRCEnv(tt.in), "quoteOpenRCEnv(%q)", tt.in)
	}
}

func TestOpenRCEnable_AddsDefaultRunlevel(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	r.When("rc-update", []string{"add", "k0scontroller", "default"}, reply{exit: 0})

	svc := newOpenRC("k0scontroller")
	svc.root = root
	svc.runner = r

	require.NoError(t, svc.Enable(ctx))
	require.Len(t, r.calls, 1)
	assert.Equal(t, "rc-update add k0scontroller default", r.calls[0].name+" "+strings.Join(r.calls[0].args, " "))
}

func TestOpenRCStart_CallsRcServiceStart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "etc/init.d/k0sworker"), []byte("#!/sbin/openrc-run\n"), 0o755))

	r.When("rc-service", []string{"k0sworker", "start"}, reply{exit: 0})

	svc := newOpenRC("k0sworker")
	svc.root = root
	svc.runner = r

	require.NoError(t, svc.Start(ctx))
	require.Len(t, r.calls, 1)
	assert.Equal(t, "rc-service k0sworker start", r.calls[0].name+" "+strings.Join(r.calls[0].args, " "))
}

func TestOpenRCStatus(t *testing.T) {
	tests := []struct {
		name  string
		reply reply
		want  Status
	}{
		{name: "started", reply: reply{exit: 0}, want: StatusRunning},
		{name: "starting", reply: reply{exit: 8}, want: StatusRunning},
		{name: "not installed", reply: reply{exit: 1}, want: StatusNotInstalled},
		{name: "stopped", reply: reply{exit: 3}, want: StatusStopped},
		{name: "stopping", reply: reply{exit: 4}, want: StatusRunning},
		{name: "inactive", reply: reply{exit: 16}, want: StatusStopped},
		{name: "crashed", reply: reply{exit: 32}, want: StatusStopped},
		{name: "unsupervised", reply: reply{exit: 64}, want: StatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRunner()
			r.When("rc-service", []string{"k0scontroller", "status"}, tt.reply)

			svc := newOpenRC("k0scontroller")
			svc.runner = r

			st, err := svc.Status(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.want, st)
		})
	}
}
