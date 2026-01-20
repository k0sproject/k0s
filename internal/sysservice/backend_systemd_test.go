// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdInstall_WritesUnitUnderRoot_AndReloadsDaemon(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/systemd/system"), 0o755))
	r := newFakeRunner()

	svc := newSystemd("k0scontroller")
	svc.root = root
	svc.runner = r

	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})
	r.When("systemctl", []string{"enable", "k0scontroller.service"}, reply{exit: 0})

	require.NoError(t, svc.Install(ctx, InstallOpts{
		Executable: testExecutable,
		Args:       []string{"controller", "--config", "/etc/k0s/k0s.yaml"},
		Env:        []string{"K0S_LOG_LEVEL=info", "K0S_FOO=bar baz"},
	}))

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

	assert.Equal(t, []string{"systemctl daemon-reload", "systemctl enable k0scontroller.service"}, r.commands())
}

func TestSystemdInstall_FailsIfAlreadyInstalled(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/systemd/system"), 0o755))
	r := newFakeRunner()

	svc := newSystemd("k0scontroller")
	svc.root = root
	svc.runner = r

	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})
	r.When("systemctl", []string{"enable", "k0scontroller.service"}, reply{exit: 0})

	require.NoError(t, svc.Install(ctx, InstallOpts{Executable: testExecutable}), "first Install() should succeed")
	assert.ErrorIs(t, svc.Install(ctx, InstallOpts{Executable: testExecutable}), ErrAlreadyInstalled)
}

func TestSystemdInstall_ForceReplacesUnit(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/systemd/system"), 0o755))
	r := newFakeRunner()

	svc := newSystemd("k0scontroller")
	svc.root = root
	svc.runner = r

	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})
	r.When("systemctl", []string{"enable", "k0scontroller.service"}, reply{exit: 0})

	require.NoError(t, svc.Install(ctx, InstallOpts{Executable: testExecutable, Args: []string{"worker"}}))
	require.NoError(t, svc.Install(ctx, InstallOpts{Executable: testExecutable, Args: []string{"controller"}, Force: true}))

	unitPath := filepath.Join(root, "etc/systemd/system/k0scontroller.service")
	unitBytes, err := os.ReadFile(unitPath)
	require.NoError(t, err)
	assert.Contains(t, string(unitBytes), "ExecStart=/usr/local/bin/k0s controller\n")
	assert.NotContains(t, string(unitBytes), "worker")

	if runtime.GOOS != "windows" { // no POSIX permissions there
		info, err := os.Stat(unitPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	}

	assert.Equal(t, []string{
		"systemctl daemon-reload", "systemctl enable k0scontroller.service",
		"systemctl daemon-reload", "systemctl enable k0scontroller.service",
	}, r.commands(), "the service must not be stopped or started")
}

func TestSystemdStart_Stop(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	r := newFakeRunner()

	svc := newSystemd("k0sworker")
	svc.root = root
	svc.runner = r

	r.When("systemctl", []string{"start", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"stop", "k0sworker.service"}, reply{exit: 0})

	require.NoError(t, svc.Start(ctx))
	require.NoError(t, svc.Stop(ctx))

	assert.Equal(t, []string{"systemctl start k0sworker.service", "systemctl stop k0sworker.service"}, r.commands())
}

func TestQuoteSystemdArg(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", `""`},
		{"/usr/bin/k0s", `/usr/bin/k0s`},
		{"--labels=a=b,c=d", `--labels=a=b,c=d`},
		{"has space", `"has space"`},
		{"has\ttab", `"has\ttab"`},
		{"has\nnewline", `"has\nnewline"`},
		{"bell\a", `"bell\x07"`},
		{`back\slash`, `"back\\slash"`},
		{"100%done", `"100%%done"`},
		{"$HOME", `"$$HOME"`},
		{`"quoted"`, `"\"quoted\""`},
		{`it's`, `"it's"`},
		{";", `";"`},
		{"a;b", `"a;b"`},
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
		{`KEY=$HOME`, `"KEY=$HOME"`},
		{`KEY=back\slash`, `"KEY=back\\slash"`},
		{`KEY="quoted"`, `"KEY=\"quoted\""`},
		{"KEY=first\nsecond", `"KEY=first\nsecond"`},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, quoteSystemdEnv(tt.in), "quoteSystemdEnv(%q)", tt.in)
	}
}

func TestRenderSystemdUnit_ExecPath(t *testing.T) {
	unit, err := renderSystemdUnit("/opt/k0s 100%/k0s", nil, nil)
	require.NoError(t, err)
	assert.Contains(t, string(unit), "\nConditionFileIsExecutable=/opt/k0s 100%%/k0s\n")
	assert.Contains(t, string(unit), "\nExecStart=\"/opt/k0s 100%%/k0s\"\n")

	_, err = renderSystemdUnit("/opt/k0s\n/k0s", nil, nil)
	assert.ErrorContains(t, err, "control characters")
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
		{name: "no such unit", reply: reply{exit: 4}, want: StatusUnknown, wantErr: true},
		{name: "unknown exit", reply: reply{exit: 99}, want: StatusUnknown, wantErr: true},
		{name: "exec error", reply: reply{err: errors.New("exec: not found")}, want: StatusUnknown, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRunner()
			r.When("systemctl", []string{"is-active", "--quiet", "k0scontroller.service"}, tt.reply)

			svc := newSystemd("k0scontroller")
			svc.root = t.TempDir()
			svc.runner = r

			require.NoError(t, os.MkdirAll(svc.unitDir(), 0o755))
			require.NoError(t, os.WriteFile(svc.unitPath(), []byte("[Unit]\n"), 0o644))

			st, err := svc.Status(t.Context())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, st)
		})
	}

	t.Run("not installed", func(t *testing.T) {
		r := newFakeRunner()

		svc := newSystemd("k0scontroller")
		svc.root = t.TempDir()
		svc.runner = r

		st, err := svc.Status(t.Context())
		require.NoError(t, err)
		assert.Equal(t, StatusNotInstalled, st)
		assert.Empty(t, r.calls, "systemctl should not be called without a unit file")
	})
}

func TestSystemdUninstall_RemovesUnit_AndReloadsDaemon(t *testing.T) {
	ctx := t.Context()
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

	assert.Equal(t, []string{"systemctl disable k0scontroller.service", "systemctl daemon-reload"}, r.commands())
}

func TestSystemdUninstall_KeepsUnitIfDisableFails(t *testing.T) {
	r := newFakeRunner()
	svc := newSystemd("k0scontroller")
	svc.root = t.TempDir()
	svc.runner = r

	require.NoError(t, os.MkdirAll(svc.unitDir(), 0o755))
	require.NoError(t, os.WriteFile(svc.unitPath(), []byte("[Unit]\n"), 0o644))

	r.When("systemctl", []string{"disable", "k0scontroller.service"}, reply{exit: 1})

	assert.Error(t, svc.Uninstall(t.Context()))
	assert.FileExists(t, svc.unitPath(), "the unit file should be kept")
}

func TestSystemdUninstall_NotInstalled(t *testing.T) {
	r := newFakeRunner()
	svc := newSystemd("k0scontroller")
	svc.root = t.TempDir()
	svc.runner = r

	assert.ErrorIs(t, svc.Uninstall(t.Context()), ErrNotInstalled)
	assert.Empty(t, r.calls)
}
