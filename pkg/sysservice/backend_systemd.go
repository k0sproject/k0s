// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
)

type systemdService struct {
	spec   Spec
	root   string
	runner Runner
}

func newSystemd(spec Spec, cfg config) *systemdService {
	return &systemdService{
		spec:   spec,
		root:   cfg.root,
		runner: cfg.runner,
	}
}

func (s *systemdService) Kind() string { return "systemd" }

func (s *systemdService) unitDir() string {
	if s.root != "" {
		return filepath.Join(s.root, "etc/systemd/system")
	}
	return filepath.Join(string(filepath.Separator), "etc/systemd/system")
}

func (s *systemdService) unitName() string {
	return s.spec.Name + ".service"
}

func (s *systemdService) unitPath() string {
	return filepath.Join(s.unitDir(), s.unitName())
}

func (s *systemdService) Install(ctx context.Context) error {
	if err := dir.Init(s.unitDir(), 0o755); err != nil {
		return err
	}

	unit, err := renderSystemdUnit(s.spec)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.unitPath(), unit, 0o644); err != nil {
		return err
	}

	return s.daemonReload(ctx)
}

func (s *systemdService) Uninstall(ctx context.Context) error {
	_ = s.Disable(ctx)
	_ = s.Stop(ctx)

	if err := os.Remove(s.unitPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return s.daemonReload(ctx)
}

func (s *systemdService) Enable(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "systemctl", "enable", s.unitName())
	return err
}

func (s *systemdService) Disable(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "systemctl", "disable", s.unitName())
	return err
}

func (s *systemdService) Start(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "systemctl", "start", s.unitName())
	return err
}

func (s *systemdService) Stop(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "systemctl", "stop", s.unitName())
	return err
}

func (s *systemdService) Restart(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "systemctl", "restart", s.unitName())
	return err
}

func (s *systemdService) Status(ctx context.Context) (Status, error) {
	if !file.Exists(s.unitPath()) {
		return StatusNotInstalled, nil
	}

	exit, _, _, err := s.runner.Run(ctx, "systemctl", "is-active", "--quiet", s.unitName())
	if err != nil {
		return StatusUnknown, err
	}
	if exit == 0 {
		return StatusRunning, nil
	}
	return StatusStopped, nil
}

func (s *systemdService) daemonReload(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "systemctl", "daemon-reload")
	return err
}

func renderSystemdUnit(spec Spec) ([]byte, error) {
	const unitTmpl = `[Unit]
Description={{.Description}}
Documentation=https://docs.k0sproject.io
ConditionFileIsExecutable={{.Exec}}
After=network-online.target
Wants=network-online.target

[Service]
StartLimitInterval=5
StartLimitBurst=10
ExecStart={{.ExecStart}}
{{- if .WorkingDir}}
WorkingDirectory={{.WorkingDir}}
{{- end}}
{{- if .User}}
User={{.User}}
{{- end}}
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
		Description string
		Exec        string
		ExecStart   string
		WorkingDir  string
		User        string
		Env         []string
	}{
		Description: spec.Description,
		Exec:        quoteSystemdArg(spec.Exec),
		ExecStart:   systemdCommandLine(spec.Exec, spec.Args),
		WorkingDir:  spec.WorkingDir,
		User:        spec.User,
		Env:         systemdEnvironment(spec.Env),
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
	return strings.ReplaceAll(arg, " ", `\x20`)
}

func quoteSystemdEnv(arg string) string {
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}
