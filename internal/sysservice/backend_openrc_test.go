// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"cmp"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRCInstall_WritesInitScript_UnderRoot(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))
	r := newFakeRunner()

	r.When("rc-update", []string{"add", "k0scontroller", "default"}, reply{})

	svc := newOpenRC("k0scontroller")
	svc.root = root
	svc.runner = r

	require.NoError(t, svc.Install(ctx, InstallOpts{
		Executable: testExecutable,
		Args:       []string{"controller", "--config", "/etc/k0s/k0s.yaml"},
		Env:        []string{"K0S_FOO=bar", "K0S_BAR=baz"},
	}))

	initPath := filepath.Join(root, "etc/init.d/k0scontroller")
	initBytes, err := os.ReadFile(initPath)
	require.NoError(t, err, "expected init script at %s", initPath)

	initTxt := string(initBytes)
	assert.Contains(t, initTxt, "command=/usr/local/bin/k0s\n")
	assert.Contains(t, initTxt, "k0s_arg_1='controller'\nk0s_arg_2='--config'\nk0s_arg_3='/etc/k0s/k0s.yaml'\n")
	assert.Contains(t, initTxt, `command_args='"$k0s_arg_1" "$k0s_arg_2" "$k0s_arg_3"'`)
	assert.Contains(t, initTxt, `supervise-daemon`)
	assert.Contains(t, initTxt, "export K0S_FOO='bar'")
	assert.Contains(t, initTxt, "export K0S_BAR='baz'")
	assert.Equal(t, []string{"rc-update add k0scontroller default"}, r.commands())
}

func TestOpenRCInstall_FailsIfAlreadyInstalled(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))
	r := newFakeRunner()

	r.When("rc-update", []string{"add", "k0scontroller", "default"}, reply{})

	svc := newOpenRC("k0scontroller")
	svc.root = root
	svc.runner = r

	require.NoError(t, svc.Install(ctx, InstallOpts{Executable: testExecutable}), "first Install() should succeed")

	assert.ErrorIs(t, svc.Install(ctx, InstallOpts{Executable: testExecutable}), ErrAlreadyInstalled)
}

func TestOpenRCInstall_ForceReplacesInitScript(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))
	r := newFakeRunner()

	r.When("rc-update", []string{"add", "k0scontroller", "default"}, reply{})

	svc := newOpenRC("k0scontroller")
	svc.root = root
	svc.runner = r

	require.NoError(t, svc.Install(ctx, InstallOpts{Executable: testExecutable, Args: []string{"worker"}}))
	require.NoError(t, svc.Install(ctx, InstallOpts{Executable: testExecutable, Args: []string{"controller"}, Force: true}))

	initPath := filepath.Join(root, "etc/init.d/k0scontroller")
	initBytes, err := os.ReadFile(initPath)
	require.NoError(t, err)
	assert.Contains(t, string(initBytes), "k0s_arg_1='controller'\n")
	assert.NotContains(t, string(initBytes), `'worker'`)

	if runtime.GOOS != "windows" { // no POSIX permissions there
		info, err := os.Stat(initPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "the init script has to stay executable")
	}

	assert.Equal(t, []string{
		"rc-update add k0scontroller default",
		"rc-update add k0scontroller default",
	}, r.commands(), "the service must not be stopped or started")
}

func TestOpenRCInstall_ArgsRoundTrip(t *testing.T) {
	// Pathname expansion depends on the cwd, so put files there that the glob
	// arguments would match if they were expanded.
	cwd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "'evil'"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "evil"), nil, 0o644))

	args := []string{
		"controller",
		"--kubelet-extra-args=--node-labels foo=bar",
		"--data-dir=/var/lib/k0s  data",
		"--token-file=/root/join\ttoken",
		"--config=/etc/k0s/k0s\nconfig.yaml",
		`--config=it's.yaml`,
		`say "hi"`,
		`back\slash`,
		"--profile=$(reboot)",
		"`reboot`",
		"$HOME",
		"*",
		"e?il",
		"",
	}
	initPath := installOpenRC(t, InstallOpts{Executable: testExecutable, Args: args})

	assert.Equal(t, args, openrcStartArgv(t, initPath, cwd))
}

func TestOpenRCInstall_EnvRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}

	env := []string{
		"K0S_PLAIN=value",
		"K0S_SPACES=has  spaces",
		"K0S_SHELL=$(reboot) `reboot` $HOME it's \"quoted\" back\\slash",
		"K0S_NEWLINE=first line\nsecond line",
		"K0S_EMPTY=",
	}
	initPath := installOpenRC(t, InstallOpts{Executable: testExecutable, Args: []string{"controller"}, Env: env})

	for _, kv := range env {
		key, val, _ := strings.Cut(kv, "=")
		// The trailing dot preserves trailing newlines.
		out, err := exec.Command("sh", "-c", `. "$1" && printenv "$2" && echo .`, "sh", initPath, key).Output()
		require.NoError(t, err)
		assert.Equal(t, val+"\n.\n", string(out), "value of %s", key)
	}
}

func TestOpenRCInstall_RejectsUnsafeInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		exec string
		env  []string
	}{
		{name: "space in exec", exec: "/opt/k0s dir/k0s"},
		{name: "glob in exec", exec: "/opt/k0s*/k0s"},
		{name: "quote in exec", exec: "/opt/it's/k0s"},
		{name: "space in env key", env: []string{"A B=x"}},
		{name: "command substitution in env key", env: []string{"$(echo INJECTED)=y"}},
		{name: "env key starting with digit", env: []string{"1A=x"}},
		{name: "env without value", env: []string{"K0S_FOO"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))

			svc := newOpenRC("k0scontroller")
			svc.root = root
			svc.runner = newFakeRunner()

			err := svc.Install(t.Context(), InstallOpts{Executable: cmp.Or(tt.exec, testExecutable), Args: []string{"controller"}, Env: tt.env})
			assert.ErrorContains(t, err, "OpenRC")
			assert.NoFileExists(t, filepath.Join(root, "etc/init.d/k0scontroller"))
		})
	}
}

// installOpenRC installs the service into a temporary root and returns the
// path of the init script.
func installOpenRC(t *testing.T, opts InstallOpts) string {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))

	r := newFakeRunner()
	r.When("rc-update", []string{"add", "k0scontroller", "default"}, reply{})

	svc := newOpenRC("k0scontroller")
	svc.root = root
	svc.runner = r

	require.NoError(t, svc.Install(t.Context(), opts))
	return filepath.Join(root, "etc/init.d/k0scontroller")
}

// openrcStartArgv sources the init script in a POSIX shell and expands
// $command and $command_args the way OpenRC's supervise-daemon.sh does:
// unquoted, into eval. Returns the arguments supervise-daemon would pass to
// the command. The shell runs in cwd, as pathname expansion depends on it.
func openrcStartArgv(t *testing.T, initPath, cwd string) []string {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}

	cmd := exec.Command("sh", "-c", `set -e; . "$1"; eval printf "'%s\\0'" $command -- $command_args`, "sh", initPath)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		require.NoError(t, err, "%s", exitErr.Stderr)
	}
	require.NoError(t, err)

	words := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	require.Equal(t, []string{"/usr/local/bin/k0s", "--"}, words[:2])
	return words[2:]
}

func TestOpenRCArgs(t *testing.T) {
	vars, refs := openrcArgs([]string{"controller", "it's", ""})
	assert.Equal(t, []string{"k0s_arg_1='controller'", `k0s_arg_2='it'\''s'`, "k0s_arg_3=''"}, vars)
	assert.Equal(t, `"$k0s_arg_1" "$k0s_arg_2" "$k0s_arg_3"`, refs)

	vars, refs = openrcArgs(nil)
	assert.Empty(t, vars)
	assert.Empty(t, refs)
}

func TestQuoteOpenRCEnv(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`KEY=value`, `KEY='value'`},
		{`KEY=has space`, `KEY='has space'`},
		{`KEY=$dollar`, `KEY='$dollar'`},
		{`KEY=$(subshell)`, `KEY='$(subshell)'`},
		{`KEY=it's`, `KEY='it'\''s'`},
		{`_key1=a=b`, `_key1='a=b'`},
	}
	for _, tt := range tests {
		got, err := quoteOpenRCEnv(tt.in)
		if assert.NoError(t, err, "quoteOpenRCEnv(%q)", tt.in) {
			assert.Equal(t, tt.want, got, "quoteOpenRCEnv(%q)", tt.in)
		}
	}
}

func TestOpenRCUninstall(t *testing.T) {
	// Sets up an installed service, optionally added to the default runlevel.
	setup := func(t *testing.T, inRunlevel bool) (*openrcService, *fakeRunner) {
		root := t.TempDir()
		r := newFakeRunner()

		svc := newOpenRC("k0scontroller")
		svc.root = root
		svc.runner = r

		require.NoError(t, os.MkdirAll(svc.initDir(), 0o755))
		require.NoError(t, os.WriteFile(svc.initPath(), []byte("#!/sbin/openrc-run\n"), 0o755))
		if inRunlevel {
			require.NoError(t, os.MkdirAll(filepath.Dir(svc.runlevelPath()), 0o755))
			require.NoError(t, os.WriteFile(svc.runlevelPath(), nil, 0o644))
		}
		return svc, r
	}

	t.Run("in runlevel", func(t *testing.T) {
		svc, r := setup(t, true)
		r.When("rc-update", []string{"del", "k0scontroller", "default"}, reply{})

		require.NoError(t, svc.Uninstall(t.Context()))
		assert.NoFileExists(t, svc.initPath())
		assert.Equal(t, []string{"rc-update del k0scontroller default"}, r.commands())
	})

	t.Run("not in runlevel", func(t *testing.T) {
		svc, r := setup(t, false)

		require.NoError(t, svc.Uninstall(t.Context()))
		assert.NoFileExists(t, svc.initPath())
		assert.Empty(t, r.calls)
	})

	t.Run("rc-update fails", func(t *testing.T) {
		svc, r := setup(t, true)
		r.When("rc-update", []string{"del", "k0scontroller", "default"}, reply{exit: 1})

		assert.Error(t, svc.Uninstall(t.Context()))
		assert.FileExists(t, svc.initPath(), "the init script should be kept")
	})

	t.Run("not installed", func(t *testing.T) {
		r := newFakeRunner()
		svc := newOpenRC("k0scontroller")
		svc.root = t.TempDir()
		svc.runner = r

		assert.ErrorIs(t, svc.Uninstall(t.Context()), ErrNotInstalled)
		assert.Empty(t, r.calls)
	})
}

func TestOpenRCStart_CallsRcServiceStart(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	r := newFakeRunner()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "etc/init.d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "etc/init.d/k0sworker"), []byte("#!/sbin/openrc-run\n"), 0o755))

	r.When("rc-service", []string{"k0sworker", "start"}, reply{exit: 0})

	svc := newOpenRC("k0sworker")
	svc.root = root
	svc.runner = r

	require.NoError(t, svc.Start(ctx))
	require.Len(t, r.calls, 1)
	assert.Equal(t, "rc-service k0sworker start", r.calls[0].name+" "+strings.Join(r.calls[0].args, " "))
}

func TestOpenRCStatus(t *testing.T) {
	tests := []struct {
		name    string
		reply   reply
		want    Status
		wantErr bool
	}{
		{name: "started", reply: reply{exit: 0}, want: StatusRunning},
		{name: "starting", reply: reply{exit: 8}, want: StatusRunning},
		{name: "stopped", reply: reply{exit: 3}, want: StatusStopped},
		{name: "stopping", reply: reply{exit: 4}, want: StatusRunning},
		{name: "inactive", reply: reply{exit: 16}, want: StatusStopped},
		{name: "crashed", reply: reply{exit: 32}, want: StatusStopped},
		{name: "rc-service failure", reply: reply{exit: 1}, want: StatusUnknown, wantErr: true},
		{name: "unsupervised", reply: reply{exit: 64}, want: StatusUnknown, wantErr: true},
		{name: "exec error", reply: reply{err: errors.New("exec: not found")}, want: StatusUnknown, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRunner()
			r.When("rc-service", []string{"k0scontroller", "status"}, tt.reply)

			svc := newOpenRC("k0scontroller")
			svc.root = t.TempDir()
			svc.runner = r

			require.NoError(t, os.MkdirAll(svc.initDir(), 0o755))
			require.NoError(t, os.WriteFile(svc.initPath(), []byte("#!/sbin/openrc-run\n"), 0o755))

			st, err := svc.Status(t.Context())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, st)
		})
	}

	t.Run("not installed", func(t *testing.T) {
		r := newFakeRunner()

		svc := newOpenRC("k0scontroller")
		svc.root = t.TempDir()
		svc.runner = r

		st, err := svc.Status(t.Context())
		require.NoError(t, err)
		assert.Equal(t, StatusNotInstalled, st)
		assert.Empty(t, r.calls, "rc-service should not be called without an init script")
	})
}
