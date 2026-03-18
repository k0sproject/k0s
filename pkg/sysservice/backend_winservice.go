// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"fmt"
	"strings"
)

type windowsService struct {
	spec   Spec
	runner Runner
}

func newWindows(spec Spec, cfg config) *windowsService {
	return &windowsService{
		spec:   spec,
		runner: cfg.runner,
	}
}

func (s *windowsService) Kind() string { return "windows" }

func (s *windowsService) Install(ctx context.Context) error {
	_, _, _, err := s.runner.Run(
		ctx,
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		s.installScript(),
	)
	return err
}

func (s *windowsService) Uninstall(ctx context.Context) error {
	// Best effort stop before deleting the service.
	_ = s.Stop(ctx)

	_, _, _, err := s.runner.Run(ctx, "sc.exe", "delete", s.spec.Name)
	return err
}

func (s *windowsService) Enable(ctx context.Context) error {
	return s.setStartupType(ctx, "Automatic")
}

func (s *windowsService) Disable(ctx context.Context) error {
	return s.setStartupType(ctx, "Disabled")
}

func (s *windowsService) Start(ctx context.Context) error {
	return s.runPowerShell(ctx, fmt.Sprintf("Start-Service -Name %s -ErrorAction Stop", quotePowerShell(s.spec.Name)))
}

func (s *windowsService) Stop(ctx context.Context) error {
	return s.runPowerShell(ctx, fmt.Sprintf("Stop-Service -Name %s -Force -ErrorAction Stop", quotePowerShell(s.spec.Name)))
}

func (s *windowsService) Restart(ctx context.Context) error {
	return s.runPowerShell(ctx, fmt.Sprintf("Restart-Service -Name %s -Force -ErrorAction Stop", quotePowerShell(s.spec.Name)))
}

func (s *windowsService) Status(ctx context.Context) (Status, error) {
	exit, _, _, err := s.runner.Run(
		ctx,
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		s.statusScript(),
	)
	if err != nil {
		return StatusUnknown, err
	}

	switch exit {
	case 0:
		return StatusRunning, nil
	case 1:
		return StatusStopped, nil
	case 3:
		return StatusNotInstalled, nil
	default:
		return StatusUnknown, nil
	}
}

func (s *windowsService) installScript() string {
	displayName := s.spec.DisplayName
	if displayName == "" {
		displayName = s.spec.Name
	}

	script := []string{
		"$binaryPath = " + quotePowerShell(s.windowsCommandLine()),
		fmt.Sprintf("New-Service -Name %s -BinaryPathName $binaryPath -DisplayName %s -StartupType Manual -Description %s",
			quotePowerShell(s.spec.Name),
			quotePowerShell(displayName),
			quotePowerShell(s.spec.Description),
		),
	}

	return strings.Join(script, "; ")
}

func (s *windowsService) statusScript() string {
	return strings.Join([]string{
		fmt.Sprintf("$svc = Get-Service -Name %s -ErrorAction SilentlyContinue", quotePowerShell(s.spec.Name)),
		"if (-not $svc) { exit 3 }",
		"if ($svc.Status -eq 'Running') { exit 0 }",
		"if ($svc.Status -eq 'Stopped') { exit 1 }",
		"exit 2",
	}, "; ")
}

func (s *windowsService) setStartupType(ctx context.Context, startupType string) error {
	return s.runPowerShell(
		ctx,
		fmt.Sprintf("Set-Service -Name %s -StartupType %s -ErrorAction Stop", quotePowerShell(s.spec.Name), startupType),
	)
}

func (s *windowsService) runPowerShell(ctx context.Context, script string) error {
	_, _, _, err := s.runner.Run(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	return err
}

func (s *windowsService) windowsCommandLine() string {
	args := []string{quoteWindowsArg(s.spec.Exec), quoteWindowsArg("service=" + s.spec.Name)}
	for _, arg := range s.spec.Args {
		args = append(args, quoteWindowsArg(arg))
	}
	return strings.Join(args, " ")
}

func quotePowerShell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func quoteWindowsArg(s string) string {
	if s == "" {
		return `""`
	}

	if !strings.ContainsAny(s, " \t\"") {
		return s
	}

	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for _, r := range s {
		if r == '\\' {
			backslashes++
			continue
		}
		if r == '"' {
			b.WriteString(strings.Repeat(`\`, backslashes*2+1))
			b.WriteByte('"')
			backslashes = 0
			continue
		}
		if backslashes > 0 {
			b.WriteString(strings.Repeat(`\`, backslashes))
			backslashes = 0
		}
		b.WriteRune(r)
	}
	if backslashes > 0 {
		b.WriteString(strings.Repeat(`\`, backslashes*2))
	}
	b.WriteByte('"')
	return b.String()
}
