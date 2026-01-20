// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/k0sproject/k0s/internal/pkg/file"
)

type openrcService struct {
	name   string
	root   string
	runner runner
	exec   func() (string, error)
}

func newOpenRC(name string) *openrcService {
	return &openrcService{
		name:   name,
		runner: execRunner{},
		exec:   os.Executable,
	}
}

func (s *openrcService) initDir() string {
	return filepath.Join(cmp.Or(s.root, string(filepath.Separator)), "etc/init.d")
}

func (s *openrcService) initPath() string { return filepath.Join(s.initDir(), s.name) }

func (s *openrcService) Install(ctx context.Context, args []string, env []string) error {
	exec, err := s.exec()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	initScript, err := renderOpenRCInit(exec, args, env)
	if err != nil {
		return err
	}
	if err := file.WriteNew(s.initPath(), initScript, 0o755); err != nil {
		return err
	}
	return nil
}

func (s *openrcService) Uninstall(ctx context.Context) error {
	// Disable is best-effort: the service may not be enabled, in which case
	// rc-update will fail. That's fine — we still want to remove the init script.
	_ = s.Disable(ctx)

	return os.Remove(s.initPath())
}

func (s *openrcService) Enable(ctx context.Context) error {
	// Use "default" runlevel (common expectation).
	return s.runner.Run(ctx, "rc-update", "add", s.name, "default")
}

func (s *openrcService) Disable(ctx context.Context) error {
	return s.runner.Run(ctx, "rc-update", "del", s.name, "default")
}

func (s *openrcService) Start(ctx context.Context) error {
	return s.runner.Run(ctx, "rc-service", s.name, "start")
}

func (s *openrcService) Stop(ctx context.Context) error {
	return s.runner.Run(ctx, "rc-service", s.name, "stop")
}

func (s *openrcService) Status(ctx context.Context) (Status, error) {
	// rc-service exit codes:
	// 0=started
	// 1=does not exist
	//   - https://github.com/OpenRC/openrc/blob/2e1dbd9ebe08f7dbdf33afd29a5c561fa173f3e8/src/rc-service/rc-service.c#L156
	//   - https://github.com/OpenRC/openrc/blob/2e1dbd9ebe08f7dbdf33afd29a5c561fa173f3e8/src/libeinfo/libeinfo.c#L663
	// 3=stopped
	// 4=stopping
	// 8=starting
	// 16=inactive
	// 32=crashed
	// 64=unsupervised
	// see: https://github.com/OpenRC/openrc/blob/2e1dbd9ebe08f7dbdf33afd29a5c561fa173f3e8/sh/openrc-run.sh.in#L139
	//      https://github.com/OpenRC/openrc/blob/2e1dbd9ebe08f7dbdf33afd29a5c561fa173f3e8/sh/supervise-daemon.sh#L88
	if err := s.runner.Run(ctx, "rc-service", s.name, "status"); err != nil {
		var exitCoder interface{ ExitCode() int }
		if errors.As(err, &exitCoder) {
			switch exitCoder.ExitCode() {
			case 1:
				return StatusNotInstalled, nil
			case 3, 16, 32:
				// 3=stopped, 16=inactive, 32=crashed
				return StatusStopped, nil
			case 4:
				// 4=stopping: the process is still running but shutting down.
				// Treat as running so that a repeated `k0s stop` call passes
				// through to rc-service, which is idempotent and will wait for
				// the shutdown to complete rather than returning "already stopped".
				return StatusRunning, nil
			case 8:
				// 8=starting: treat as running so callers don't attempt a
				// redundant start while the service is coming up.
				return StatusRunning, nil
			case 64:
				return StatusUnknown, nil
			}
		}
		return StatusUnknown, err
	}

	return StatusRunning, nil
}

func renderOpenRCInit(exec string, args []string, env []string) ([]byte, error) {
	const initTmpl = `#!/sbin/openrc-run
supervisor=supervise-daemon
description="k0s - Zero Friction Kubernetes"
command="{{.Exec}}"
command_args="{{range .Args}}'{{.}}' {{end}}"
name="k0s"
supervise_daemon_args="--stdout /var/log/${name}.log --stderr /var/log/${name}.err"
retry="SIGTERM/90/SIGKILL/5"

: "${rc_ulimit=-n 1048576 -u unlimited}"

{{range .Env}}
export {{.}}{{end}}

depend() {
	need cgroups
	need net
	use dns
	after firewall
}
`
	data := struct {
		Exec string
		Args []string
		Env  []string
	}{
		Exec: exec,
		Args: args,
		Env:  openrcEnvironment(env),
	}
	t, err := template.New("openrc-init").Parse(initTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func openrcEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		out = append(out, quoteOpenRCEnv(kv))
	}
	return out
}

// quoteOpenRCEnv turns KEY=some value into KEY='some value' using single quotes
// so that $var, $(...) and other shell expansions cannot occur.
func quoteOpenRCEnv(kv string) string {
	key, val, ok := strings.Cut(kv, "=")
	if !ok {
		return kv
	}
	// Embedded single quotes are handled by stepping outside the quoted string.
	val = strings.ReplaceAll(val, `'`, `'\''`)
	return key + `='` + val + `'`
}
