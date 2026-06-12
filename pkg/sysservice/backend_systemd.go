// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/k0sproject/k0s/internal/pkg/file"
)

type systemdService struct {
	name   string
	root   string
	runner runner
	exec   func() (string, error)
}

func newSystemd(name string) *systemdService {
	return &systemdService{
		name:   name,
		runner: execRunner{},
		exec:   os.Executable,
	}
}

func (s *systemdService) unitDir() string {
	return filepath.Join(cmp.Or(s.root, string(filepath.Separator)), "etc/systemd/system")
}

func (s *systemdService) unitName() string {
	return s.name + ".service"
}

func (s *systemdService) unitPath() string {
	return filepath.Join(s.unitDir(), s.unitName())
}

func (s *systemdService) Install(ctx context.Context, args []string, env []string) error {
	exec, err := s.exec()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	unit, err := renderSystemdUnit(exec, args, env)
	if err != nil {
		return err
	}
	if err := file.WriteNew(s.unitPath(), unit, 0o644); err != nil {
		return err
	}

	return s.daemonReload(ctx)
}

func (s *systemdService) Uninstall(ctx context.Context) error {
	// Disable is best-effort: the service may not be enabled, in which case
	// systemctl will fail. That's fine — we still want to remove the unit file.
	_ = s.Disable(ctx)

	if err := os.Remove(s.unitPath()); err != nil {
		return err
	}

	return s.daemonReload(ctx)
}

func (s *systemdService) Enable(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "enable", s.unitName())
}

func (s *systemdService) Disable(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "disable", s.unitName())
}

func (s *systemdService) Start(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "start", s.unitName())
}

func (s *systemdService) Stop(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "stop", s.unitName())
}

func (s *systemdService) Status(ctx context.Context) (Status, error) {
	err := s.runner.Run(ctx, "systemctl", "is-active", "--quiet", s.unitName())
	code := exitCode(err)
	if code < 0 {
		return StatusUnknown, err
	}
	// systemctl is-active uses LSB exit codes; see:
	// https://github.com/systemd/systemd/blob/main/src/systemctl/systemctl-is-active.c
	switch code {
	case 0:
		// active, reloading, or refreshing — service is running
		return StatusRunning, nil
	case 3:
		// inactive, activating, deactivating, or failed — service is not running
		return StatusStopped, nil
	case 4:
		// unit file does not exist
		return StatusNotInstalled, nil
	default:
		return StatusUnknown, nil
	}
}

func (s *systemdService) daemonReload(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "daemon-reload")
}

func renderSystemdUnit(exec string, args []string, env []string) ([]byte, error) {
	const unitTmpl = `[Unit]
Description=k0s - Zero Friction Kubernetes
Documentation=https://docs.k0sproject.io
ConditionFileIsExecutable={{.Exec}}
After=network-online.target
Wants=network-online.target

[Service]
StartLimitInterval=5
StartLimitBurst=10
ExecStart={{.ExecStart}}
{{- range .Env}}
Environment={{.}}
{{- end}}
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

	data := struct {
		Exec      string
		ExecStart string
		Env       []string
	}{
		Exec:      quoteSystemdArg(exec),
		ExecStart: systemdCommandLine(exec, args),
		Env:       systemdEnvironment(env),
	}

	t, err := template.New("systemd-unit").Parse(unitTmpl)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func systemdCommandLine(execPath string, args []string) string {
	parts := []string{quoteSystemdArg(execPath)}
	for _, arg := range args {
		parts = append(parts, quoteSystemdArg(arg))
	}
	return strings.Join(parts, " ")
}

func systemdEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		out = append(out, quoteSystemdEnv(kv))
	}
	return out
}

func quoteSystemdArg(arg string) string {
	if arg == "" {
		return `""`
	}
	// Escape backslash first, then quote and percent (systemd expands %specifiers).
	arg = strings.ReplaceAll(arg, `\`, `\\`)
	arg = strings.ReplaceAll(arg, `"`, `\"`)
	arg = strings.ReplaceAll(arg, "%", "%%")
	if strings.ContainsAny(arg, " \t") {
		return `"` + arg + `"`
	}
	return arg
}

func quoteSystemdEnv(kv string) string {
	key, val, ok := strings.Cut(kv, "=")
	if !ok {
		return `"` + kv + `"`
	}
	val = strings.ReplaceAll(val, `\`, `\\`)
	val = strings.ReplaceAll(val, `"`, `\"`)
	val = strings.ReplaceAll(val, "%", "%%")
	return `"` + key + "=" + val + `"`
}
