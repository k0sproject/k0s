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

func TestOpenRCInstall_WritesInitAndConf_UnderRoot_AndAddsDefaultRunlevel(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	r.When("rc-update", []string{"add", "k0scontroller", "default"}, reply{exit: 0})

	spec := Spec{
		Name:        "k0scontroller",
		Description: "k0s controller",
		Exec:        "/usr/local/bin/k0s",
		Args:        []string{"controller", "--config", "/etc/k0s/k0s.yaml"},
		Env:         []string{"K0S_LOG_LEVEL=info", "K0S_FOO=bar"},
		User:        "root",
		Autostart:   true,
	}

	svc, err := New(spec, WithKind("openrc"), WithRoot(root), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := svc.Install(ctx); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	initPath := filepath.Join(root, "etc/init.d/k0scontroller")

	initBytes, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("expected init script at %s: %v", initPath, err)
	}

	initTxt := string(initBytes)
	if !strings.Contains(initTxt, `command="/usr/local/bin/k0s"`) {
		t.Fatalf("init script missing command, got:\n%s", initTxt)
	}
	if !strings.Contains(initTxt, `command_args="'controller' '--config' '/etc/k0s/k0s.yaml' "`) {
		t.Fatalf("init script missing args, got:\n%s", initTxt)
	}
	if !strings.Contains(initTxt, `supervise-daemon`) {
		t.Fatalf("init script missing supervise-daemon, got:\n%s", initTxt)
	}

	// verify command calls
	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
	if got := r.calls[0].name + " " + strings.Join(r.calls[0].args, " "); got != "rc-update add k0scontroller default" {
		t.Fatalf("unexpected call: %s", got)
	}
}

func TestOpenRCStart_CallsRcServiceStart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	// Pretend installed
	if err := os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/init.d/k0sworker"), []byte("#!/sbin/openrc-run\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	r.When("rc-service", []string{"k0sworker", "start"}, reply{exit: 0})

	spec := Spec{Name: "k0sworker", Description: "k0s worker", Exec: "/usr/local/bin/k0s", Args: []string{"worker"}}
	svc, err := New(spec, WithKind("openrc"), WithRoot(root), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
	if got := r.calls[0].name + " " + strings.Join(r.calls[0].args, " "); got != "rc-service k0sworker start" {
		t.Fatalf("unexpected call: %s", got)
	}
}

func TestOpenRCStatus_NotInstalledWhenInitMissing(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	spec := Spec{Name: "k0scontroller", Description: "k0s controller", Exec: "/usr/local/bin/k0s", Args: []string{"controller"}}
	svc, err := New(spec, WithKind("openrc"), WithRoot(root), WithRunner(r))
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

func TestOpenRCStatus_RunningWhenRcServiceExit0(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	r := newFakeRunner()

	// Pretend installed
	if err := os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/init.d/k0scontroller"), []byte("#!/sbin/openrc-run\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	r.When("rc-service", []string{"k0scontroller", "status"}, reply{exit: 0, stdout: "started"})

	spec := Spec{Name: "k0scontroller", Description: "k0s controller", Exec: "/usr/local/bin/k0s", Args: []string{"controller"}}
	svc, err := New(spec, WithKind("openrc"), WithRoot(root), WithRunner(r))
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
