// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"strings"
	"testing"
)

func TestWindowsInstall_CreatesService(t *testing.T) {
	ctx := context.Background()
	r := newFakeRunner()
	spec := Spec{
		Name:        "k0sworker",
		DisplayName: "k0s worker",
		Description: "k0s worker service",
		Exec:        `C:\Program Files\k0s\k0s.exe`,
		Args:        []string{"worker", "--token-file", `C:\ProgramData\k0s\token.txt`},
	}

	svc, err := New(spec, WithKind("windows"), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	winSvc, ok := svc.(*windowsService)
	if !ok {
		t.Fatalf("expected *windowsService, got %T", svc)
	}

	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", winSvc.installScript()}, reply{exit: 0})

	if err := svc.Install(ctx); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
	if r.calls[0].name != "powershell" {
		t.Fatalf("expected powershell call, got %q", r.calls[0].name)
	}
	gotScript := r.calls[0].args[len(r.calls[0].args)-1]
	if !strings.Contains(gotScript, "New-Service -Name 'k0sworker'") {
		t.Fatalf("install script missing service name, got %q", gotScript)
	}
	if !strings.Contains(gotScript, `"C:\Program Files\k0s\k0s.exe"`) {
		t.Fatalf("install script missing executable, got %q", gotScript)
	}
	if !strings.Contains(gotScript, `service=k0sworker`) {
		t.Fatalf("install script missing service arg, got %q", gotScript)
	}
}

func TestWindowsEnable_Disable_Start_Stop_Restart(t *testing.T) {
	ctx := context.Background()
	r := newFakeRunner()
	spec := Spec{Name: "k0scontroller", Exec: `C:\k0s.exe`}

	svc, err := New(spec, WithKind("windows"), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", "Set-Service -Name 'k0scontroller' -StartupType Automatic -ErrorAction Stop"}, reply{exit: 0})
	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", "Set-Service -Name 'k0scontroller' -StartupType Disabled -ErrorAction Stop"}, reply{exit: 0})
	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", "Start-Service -Name 'k0scontroller' -ErrorAction Stop"}, reply{exit: 0})
	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", "Stop-Service -Name 'k0scontroller' -Force -ErrorAction Stop"}, reply{exit: 0})
	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", "Restart-Service -Name 'k0scontroller' -Force -ErrorAction Stop"}, reply{exit: 0})

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

func TestWindowsUninstall_StopsAndDeletesService(t *testing.T) {
	ctx := context.Background()
	r := newFakeRunner()
	spec := Spec{Name: "k0scontroller", Exec: `C:\k0s.exe`}

	svc, err := New(spec, WithKind("windows"), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", "Stop-Service -Name 'k0scontroller' -Force -ErrorAction Stop"}, reply{exit: 0})
	r.When("sc.exe", []string{"delete", "k0scontroller"}, reply{exit: 0})

	if err := svc.Uninstall(ctx); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	if len(r.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(r.calls))
	}
}

func TestWindowsStatus(t *testing.T) {
	tests := []struct {
		name  string
		reply reply
		want  Status
	}{
		{name: "running", reply: reply{exit: 0}, want: StatusRunning},
		{name: "stopped", reply: reply{exit: 1}, want: StatusStopped},
		{name: "not installed", reply: reply{exit: 3}, want: StatusNotInstalled},
		{name: "unknown", reply: reply{exit: 2}, want: StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			r := newFakeRunner()
			spec := Spec{Name: "k0scontroller", Exec: `C:\k0s.exe`}

			svc, err := New(spec, WithKind("windows"), WithRunner(r))
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			winSvc := svc.(*windowsService)
			r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", winSvc.statusScript()}, tt.reply)

			got, err := svc.Status(ctx)
			if err != nil {
				t.Fatalf("Status() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestWindowsStatus_ReturnsRunnerError(t *testing.T) {
	ctx := context.Background()
	r := newFakeRunner()
	spec := Spec{Name: "k0scontroller", Exec: `C:\k0s.exe`}

	svc, err := New(spec, WithKind("windows"), WithRunner(r))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	winSvc := svc.(*windowsService)
	r.When("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", winSvc.statusScript()}, reply{exit: 1, err: context.DeadlineExceeded})

	_, err = svc.Status(ctx)
	if err == nil {
		t.Fatal("expected Status() error")
	}
}
