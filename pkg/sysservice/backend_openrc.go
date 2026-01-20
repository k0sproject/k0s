// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"text/template"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
)

type openrcService struct {
	spec   Spec
	root   string
	runner Runner
}

func newOpenRC(spec Spec, cfg config) *openrcService {
	return &openrcService{
		spec:   spec,
		root:   cfg.root,
		runner: cfg.runner,
	}
}

func (s *openrcService) Kind() string { return "openrc" }

func (s *openrcService) initDir() string {
	if s.root != "" {
		return filepath.Join(s.root, "etc/init.d")
	}
	return filepath.Join(string(filepath.Separator), "etc/init.d")
}

func (s *openrcService) initPath() string { return filepath.Join(s.initDir(), s.spec.Name) }

func (s *openrcService) Install(ctx context.Context) error {
	if err := dir.Init(s.initDir(), 0o755); err != nil {
		return err
	}
	initScript, err := renderOpenRCInit(s.spec)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.initPath(), initScript, 0o755); err != nil {
		return err
	}

	if s.spec.Autostart {
		// Use "default" runlevel (common expectation)
		if _, _, _, err := s.runner.Run(ctx, "rc-update", "add", s.spec.Name, "default"); err != nil {
			return err
		}
	}
	return nil
}

func (s *openrcService) Uninstall(ctx context.Context) error {
	// best-effort disable
	_, _, _, _ = s.runner.Run(ctx, "rc-update", "del", s.spec.Name, "default")

	if err := os.Remove(s.initPath()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (s *openrcService) Start(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "rc-service", s.spec.Name, "start")
	return err
}

func (s *openrcService) Stop(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "rc-service", s.spec.Name, "stop")
	return err
}

func (s *openrcService) Restart(ctx context.Context) error {
	_, _, _, err := s.runner.Run(ctx, "rc-service", s.spec.Name, "restart")
	return err
}

func (s *openrcService) Status(ctx context.Context) (Status, error) {
	if !file.Exists(s.initPath()) {
		return StatusNotInstalled, nil
	}
	exit, _, _, err := s.runner.Run(ctx, "rc-service", s.spec.Name, "status")
	if err != nil {
		return StatusUnknown, err
	}
	if exit == 0 {
		return StatusRunning, nil
	}
	return StatusStopped, nil
}

func renderOpenRCInit(spec Spec) ([]byte, error) {
	const initTmpl = `#!/sbin/openrc-run
supervisor=supervise-daemon
description="{{.Description}}"
command="{{.Exec}}"
{{- if .Args }}
command_args="{{range .Args}}'{{.}}' {{end}}"
{{- end }}
name=$(basename $(readlink -f $command))
supervise_daemon_args="--stdout /var/log/${name}.log --stderr /var/log/${name}.err"

: "${rc_ulimit=-n 1048576 -u unlimited}"

{{- if .Env}}{{range .Env}}
export {{.}}{{end}}{{- end}}

depend() {
	need cgroups
	need net
	use dns
	after firewall
}
`
	t, err := template.New("openrc-init").Parse(initTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, spec); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
