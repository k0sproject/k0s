// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdInstall_WritesUnitUnderRoot_AndReloadsDaemon(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/systemd/system"), 0o755))
	r := newFakeRunner()

	svc := newSystemd("k0scontroller")
	svc.root = root
	svc.runner = r
	svc.exec = func() (string, error) { return "/usr/local/bin/k0s", nil }

	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})

	require.NoError(t, svc.Install(ctx, []string{"controller", "--config", "/etc/k0s/k0s.yaml"}, []string{"K0S_LOG_LEVEL=info", "K0S_FOO=bar baz"}))

	unitPath := filepath.Join(root, "etc/systemd/system/k0scontroller.service")
	unitBytes, err := os.ReadFile(unitPath)
	require.NoError(t, err, "expected unit file at %s", unitPath)

	assert.Equal(t, `[Unit]
Description=k0s - Zero Friction Kubernetes
Documentation=https://docs.k0sproject.io
ConditionFileIsExecutable=/usr/local/bin/k0s
After=network-online.target
Wants=network-online.target

[Service]
StartLimitInterval=5
StartLimitBurst=10
ExecStart=/usr/local/bin/k0s controller --config /etc/k0s/k0s.yaml
Environment="K0S_LOG_LEVEL=info"
Environment="K0S_FOO=bar baz"
RestartSec=10
Delegate=yes
KillMode=process
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0
LimitNOFILE=999999
Restart=always

[Install]
WantedBy=multi-user.target
`, string(unitBytes))

	require.Len(t, r.calls, 1)
	assert.Equal(t, "systemctl daemon-reload", r.calls[0].name+" "+strings.Join(r.calls[0].args, " "))
}

func TestSystemdInstall_FailsIfAlreadyInstalled(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/systemd/system"), 0o755))
	r := newFakeRunner()

	svc := newSystemd("k0scontroller")
	svc.root = root
	svc.runner = r
	svc.exec = func() (string, error) { return "/usr/local/bin/k0s", nil }

	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})

	require.NoError(t, svc.Install(ctx, nil, nil), "first Install() should succeed")
	assert.ErrorIs(t, svc.Install(ctx, nil, nil), fs.ErrExist)
}

func TestSystemdEnable_Disable_Start_Stop(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	svc := newSystemd("k0sworker")
	svc.root = root
	svc.runner = r

	r.When("systemctl", []string{"enable", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"disable", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"start", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"stop", "k0sworker.service"}, reply{exit: 0})

	require.NoError(t, svc.Enable(ctx))
	require.NoError(t, svc.Disable(ctx))
	require.NoError(t, svc.Start(ctx))
	require.NoError(t, svc.Stop(ctx))

	assert.Len(t, r.calls, 4)
}

func TestQuoteSystemdArg(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", `""`},
		{"/usr/bin/k0s", `/usr/bin/k0s`},
		{"has space", `"has space"`},
		{"has\ttab", "\"has\ttab\""},
		{`back\slash`, `back\\slash`},
		{"100%done", `100%%done`},
		{`"quoted"`, `\"quoted\"`},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, quoteSystemdArg(tt.in), "quoteSystemdArg(%q)", tt.in)
	}
}

func TestQuoteSystemdEnv(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`KEY=value`, `"KEY=value"`},
		{`KEY=has space`, `"KEY=has space"`},
		{`KEY=100%done`, `"KEY=100%%done"`},
		{`KEY=back\slash`, `"KEY=back\\slash"`},
		{`KEY="quoted"`, `"KEY=\"quoted\""`},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, quoteSystemdEnv(tt.in), "quoteSystemdEnv(%q)", tt.in)
	}
}

func TestSystemdStatus(t *testing.T) {
	tests := []struct {
		name    string
		reply   reply
		want    Status
		wantErr bool
	}{
		{name: "active", reply: reply{exit: 0}, want: StatusRunning},
		{name: "not running", reply: reply{exit: 3}, want: StatusStopped},
		{name: "not installed", reply: reply{exit: 4}, want: StatusNotInstalled},
		{name: "unknown exit", reply: reply{exit: 99}, want: StatusUnknown},
		{name: "exec error", reply: reply{exit: 0, err: errors.New("exec: not found")}, want: StatusUnknown, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRunner()
			r.When("systemctl", []string{"is-active", "--quiet", "k0scontroller.service"}, tt.reply)

			svc := newSystemd("k0scontroller")
			svc.runner = r

			st, err := svc.Status(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, st)
		})
	}
}

func TestSystemdUninstall_RemovesUnit_AndReloadsDaemon(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	svc := newSystemd("k0scontroller")
	svc.root = root
	svc.runner = r

	unitPath := filepath.Join(root, "etc/systemd/system/k0scontroller.service")
	require.NoError(t, os.MkdirAll(filepath.Dir(unitPath), 0o755))
	require.NoError(t, os.WriteFile(unitPath, []byte("[Unit]\n"), 0o644))

	r.When("systemctl", []string{"disable", "k0scontroller.service"}, reply{exit: 0})
	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})

	require.NoError(t, svc.Uninstall(ctx))

	_, err := os.Stat(unitPath)
	assert.True(t, os.IsNotExist(err), "expected unit file to be removed")

	assert.Len(t, r.calls, 2)
}
