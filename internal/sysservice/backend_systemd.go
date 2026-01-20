// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"
)

type systemdService struct {
	name   string
	root   string
	runner runner
}

func newSystemd(name string) *systemdService {
	return &systemdService{
		name:   name,
		runner: execRunner{},
	}
}

func (s *systemdService) unitDir() string {
	return filepath.Join(cmp.Or(s.root, string(filepath.Separator)), "etc", "systemd", "system")
}

func (s *systemdService) unitName() string {
	return s.name + ".service"
}

func (s *systemdService) unitPath() string {
	return filepath.Join(s.unitDir(), s.unitName())
}

func (s *systemdService) Install(ctx context.Context, opts InstallOpts) error {
	unit, err := renderSystemdUnit(opts.Executable, opts.Args, opts.Env)
	if err != nil {
		return err
	}
	if err := writeDefinition(s.unitPath(), unit, 0o644, opts.Force); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrAlreadyInstalled, s.name)
		}
		return err
	}

	if err := s.daemonReload(ctx); err != nil {
		return err
	}
	return s.runner.Run(ctx, "systemctl", "enable", s.unitName())
}

func (s *systemdService) Uninstall(ctx context.Context) error {
	if _, err := os.Stat(s.unitPath()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotInstalled, s.name)
		}
		return err
	}

	// Disabling succeeds for units that aren't enabled, so any error is a
	// real one.
	if err := s.runner.Run(ctx, "systemctl", "disable", s.unitName()); err != nil {
		return err
	}
	if err := os.Remove(s.unitPath()); err != nil {
		return err
	}

	return s.daemonReload(ctx)
}

func (s *systemdService) Start(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "start", s.unitName())
}

func (s *systemdService) Stop(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "stop", s.unitName())
}

func (s *systemdService) Status(ctx context.Context) (Status, error) {
	// Check the unit file rather than relying on the is-active exit code for
	// missing units, which older systemd versions report as 3, the same as
	// for inactive units.
	if _, err := os.Stat(s.unitPath()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return StatusNotInstalled, nil
		}
		return StatusUnknown, err
	}

	// systemctl is-active uses LSB exit codes; see:
	// https://github.com/systemd/systemd/blob/main/src/systemctl/systemctl-is-active.c
	err := s.runner.Run(ctx, "systemctl", "is-active", "--quiet", s.unitName())
	if err == nil {
		// active, reloading or refreshing
		return StatusRunning, nil
	}
	if exitCode(err) == 3 {
		// inactive, activating, deactivating or failed
		return StatusStopped, nil
	}
	return StatusUnknown, err
}

func (s *systemdService) daemonReload(ctx context.Context) error {
	return s.runner.Run(ctx, "systemctl", "daemon-reload")
}

func renderSystemdUnit(exec string, args []string, env []string) ([]byte, error) {
	// ConditionFileIsExecutable= takes the path verbatim up to the end of the
	// line, without any unquoting or unescaping.
	if strings.ContainsFunc(exec, unicode.IsControl) {
		return nil, fmt.Errorf("executable path contains control characters: %q", exec)
	}

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
		Exec:      strings.ReplaceAll(exec, "%", "%%"),
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

// Words that systemd takes verbatim in ExecStart=.
var systemdPlainWord = regexp.MustCompile(`^[[:alnum:]/._+,:@=-]+$`)

// quoteSystemdArg quotes arg as a single word for ExecStart=, which systemd
// unquotes, unescapes and subjects to specifier and variable expansion.
func quoteSystemdArg(arg string) string {
	if systemdPlainWord.MatchString(arg) {
		return arg
	}
	return `"` + strings.ReplaceAll(escapeSystemd(arg), "$", "$$") + `"`
}

// quoteSystemdEnv quotes kv as a single assignment for Environment=, which
// systemd unquotes, unescapes and subjects to specifier expansion.
func quoteSystemdEnv(kv string) string {
	return `"` + escapeSystemd(kv) + `"`
}

// escapeSystemd escapes s for use inside a double quoted unit file value.
func escapeSystemd(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '%':
			b.WriteString("%%")
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
