// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdInstall_WritesUnitUnderRoot_AndReloadsDaemon(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	spec := Spec{
		Name:        "k0scontroller",
		Description: "k0s controller",
		Exec:        "/usr/local/bin/k0s",
		Args:        []string{"controller", "--config", "/etc/k0s/k0s.yaml"},
		Env:         []string{"K0S_LOG_LEVEL=info", "K0S_FOO=bar baz"},
		WorkingDir:  "/var/lib/k0s",
		User:        "root",
	}

	svc, err := New(spec, WithKind("systemd"), WithRoot(root), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})

	if err := svc.Install(ctx); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	unitPath := filepath.Join(root, "etc/systemd/system/k0scontroller.service")
	unitBytes, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("expected unit file at %s: %v", unitPath, err)
	}

	unitText := string(unitBytes)
	expectedUnit := `[Unit]
Description=k0s controller
Documentation=https://docs.k0sproject.io
ConditionFileIsExecutable=/usr/local/bin/k0s
After=network-online.target
Wants=network-online.target

[Service]
StartLimitInterval=5
StartLimitBurst=10
ExecStart=/usr/local/bin/k0s controller --config /etc/k0s/k0s.yaml
WorkingDirectory=/var/lib/k0s
User=root
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
`
	if unitText != expectedUnit {
		t.Fatalf("unexpected unit file:\n%s", unitText)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
	if got := r.calls[0].name + " " + strings.Join(r.calls[0].args, " "); got != "systemctl daemon-reload" {
		t.Fatalf("unexpected call: %s", got)
	}
}

func TestSystemdEnable_Disable_Start_Stop_Restart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	spec := Spec{Name: "k0sworker", Exec: "/usr/local/bin/k0s", Args: []string{"worker"}}

	svc, err := New(spec, WithKind("systemd"), WithRoot(root), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r.When("systemctl", []string{"enable", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"disable", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"start", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"stop", "k0sworker.service"}, reply{exit: 0})
	r.When("systemctl", []string{"restart", "k0sworker.service"}, reply{exit: 0})

	if err := svc.Enable(ctx); err != nil {
		t.Fatalf("Enable() error: %v", err)
	}
	if err := svc.Disable(ctx); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := svc.Restart(ctx); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}

	if len(r.calls) != 5 {
		t.Fatalf("expected 5 calls, got %d", len(r.calls))
	}
}

func TestSystemdStatus_NotInstalledWhenUnitMissing(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	spec := Spec{Name: "k0scontroller", Exec: "/usr/local/bin/k0s", Args: []string{"controller"}}
	svc, err := New(spec, WithKind("systemd"), WithRoot(root), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	st, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if st != StatusNotInstalled {
		t.Fatalf("expected StatusNotInstalled, got %v", st)
	}
}

func TestSystemdStatus_RunningWhenSystemctlExit0(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	if err := os.MkdirAll(filepath.Join(root, "etc/systemd/system"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/systemd/system/k0scontroller.service"), []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r.When("systemctl", []string{"is-active", "--quiet", "k0scontroller.service"}, reply{exit: 0})

	spec := Spec{Name: "k0scontroller", Exec: "/usr/local/bin/k0s", Args: []string{"controller"}}
	svc, err := New(spec, WithKind("systemd"), WithRoot(root), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	st, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if st != StatusRunning {
		t.Fatalf("expected StatusRunning, got %v", st)
	}
}

func TestSystemdUninstall_RemovesUnit_AndReloadsDaemon(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	spec := Spec{Name: "k0scontroller", Exec: "/usr/local/bin/k0s"}
	svc, err := New(spec, WithKind("systemd"), WithRoot(root), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	unitPath := filepath.Join(root, "etc/systemd/system/k0scontroller.service")
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unitPath, []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r.When("systemctl", []string{"disable", "k0scontroller.service"}, reply{exit: 0})
	r.When("systemctl", []string{"stop", "k0scontroller.service"}, reply{exit: 0})
	r.When("systemctl", []string{"daemon-reload"}, reply{exit: 0})

	if err := svc.Uninstall(ctx); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Fatalf("expected unit file removed, stat err=%v", err)
	}

	if len(r.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(r.calls))
	}
}
