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
)

type openrcService struct {
	name   string
	root   string
	runner runner
}

func newOpenRC(name string) *openrcService {
	return &openrcService{
		name:   name,
		runner: execRunner{},
	}
}

func (s *openrcService) initDir() string {
	return filepath.Join(cmp.Or(s.root, string(filepath.Separator)), "etc", "init.d")
}

func (s *openrcService) initPath() string { return filepath.Join(s.initDir(), s.name) }

// The runlevel in which the service gets started on boot.
const openrcRunlevel = "default"

func (s *openrcService) runlevelPath() string {
	return filepath.Join(cmp.Or(s.root, string(filepath.Separator)), "etc", "runlevels", openrcRunlevel, s.name)
}

func (s *openrcService) Install(ctx context.Context, opts InstallOpts) error {
	initScript, err := renderOpenRCInit(opts.Executable, opts.Args, opts.Env)
	if err != nil {
		return err
	}
	if err := writeDefinition(s.initPath(), initScript, 0o755, opts.Force); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrAlreadyInstalled, s.name)
		}
		return err
	}
	return s.runner.Run(ctx, "rc-update", "add", s.name, openrcRunlevel)
}

func (s *openrcService) Uninstall(ctx context.Context) error {
	if _, err := os.Stat(s.initPath()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotInstalled, s.name)
		}
		return err
	}

	// rc-update fails the same way for services that aren't in the runlevel
	// as for real errors. Check the runlevel's symlink instead, which is what
	// rc-update does, too.
	if _, err := os.Lstat(s.runlevelPath()); err == nil {
		if err := s.runner.Run(ctx, "rc-update", "del", s.name, openrcRunlevel); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return os.Remove(s.initPath())
}

func (s *openrcService) Start(ctx context.Context) error {
	return s.runner.Run(ctx, "rc-service", s.name, "start")
}

func (s *openrcService) Stop(ctx context.Context) error {
	return s.runner.Run(ctx, "rc-service", s.name, "stop")
}

func (s *openrcService) Status(ctx context.Context) (Status, error) {
	// Check the init script rather than relying on rc-service exiting with 1
	// for missing services. It does so for many other failures as well, e.g.
	// when OpenRC isn't the running init system.
	if _, err := os.Stat(s.initPath()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return StatusNotInstalled, nil
		}
		return StatusUnknown, err
	}

	// rc-service exit codes:
	// 0=started
	// 3=stopped
	// 4=stopping
	// 8=starting
	// 16=inactive
	// 32=crashed
	// 64=unsupervised
	// see: https://github.com/OpenRC/openrc/blob/2e1dbd9ebe08f7dbdf33afd29a5c561fa173f3e8/sh/openrc-run.sh.in#L139
	//      https://github.com/OpenRC/openrc/blob/2e1dbd9ebe08f7dbdf33afd29a5c561fa173f3e8/sh/supervise-daemon.sh#L88
	err := s.runner.Run(ctx, "rc-service", s.name, "status")
	if err == nil {
		return StatusRunning, nil
	}
	switch exitCode(err) {
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
	default:
		// 64=unsupervised means the supervisor state is inconsistent.
		return StatusUnknown, err
	}
}

func renderOpenRCInit(exec string, args []string, env []string) ([]byte, error) {
	const initTmpl = `#!/sbin/openrc-run
supervisor=supervise-daemon
description="k0s - Zero Friction Kubernetes"
command={{.Exec}}
{{- range .ArgVars}}
{{.}}{{end}}
command_args='{{.Args}}'
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
	// OpenRC expands command unquoted and evals the result, so the path can't
	// be quoted in a way that survives. Paths that need quoting are rejected.
	if !openrcPlainWord.MatchString(exec) {
		return nil, fmt.Errorf("executable path contains characters unsupported by OpenRC: %q", exec)
	}
	argVars, argRefs := openrcArgs(args)
	envVars, err := openrcEnvironment(env)
	if err != nil {
		return nil, err
	}

	data := struct {
		Exec    string
		ArgVars []string
		Args    string
		Env     []string
	}{
		Exec:    exec,
		ArgVars: argVars,
		Args:    argRefs,
		Env:     envVars,
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

// Words that pass unchanged through field splitting, pathname expansion and
// eval.
var openrcPlainWord = regexp.MustCompile(`^[[:alnum:]/._+,:@=-]+$`)

// Valid names for shell variables.
var shellName = regexp.MustCompile(`^[[:alpha:]_][[:alnum:]_]*$`)

// openrcArgs renders args for command_args. OpenRC field splits command_args,
// subjects it to pathname expansion and evals the result. No quoting survives
// all of that unscathed, so each argument goes into a variable of its own and
// command_args only references those variables in double quotes. Returns the
// variable assignments and the value for command_args.
func openrcArgs(args []string) (vars []string, refs string) {
	vars = make([]string, 0, len(args))
	quotedRefs := make([]string, 0, len(args))
	for i, arg := range args {
		name := fmt.Sprintf("k0s_arg_%d", i+1)
		vars = append(vars, name+"="+singleQuote(arg))
		quotedRefs = append(quotedRefs, `"$`+name+`"`)
	}
	return vars, strings.Join(quotedRefs, " ")
}

func openrcEnvironment(env []string) ([]string, error) {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		quoted, err := quoteOpenRCEnv(kv)
		if err != nil {
			return nil, err
		}
		out = append(out, quoted)
	}
	return out, nil
}

// quoteOpenRCEnv turns KEY=some value into KEY='some value' so that $var,
// $(...) and other shell expansions cannot occur. The shell can only export
// variables with valid names, so anything else is rejected.
func quoteOpenRCEnv(kv string) (string, error) {
	key, val, ok := strings.Cut(kv, "=")
	if !ok || !shellName.MatchString(key) {
		return "", fmt.Errorf("invalid environment variable for OpenRC: %q", kv)
	}
	return key + "=" + singleQuote(val), nil
}

// singleQuote wraps s in single quotes, stepping outside the quoted string to
// escape embedded ones.
func singleQuote(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}
