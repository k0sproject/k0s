/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubectl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
)

type KubectlSuite struct {
	common.FootlooseSuite
}

const pluginContent = `#!/bin/sh

echo "${0##*/}" "$#" "$@"
`

func (s *KubectlSuite) TestEmbeddedKubectl() {
	sshConn, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err, "failed to SSH into controller")
	defer sshConn.Disconnect()

	// Create a dummy kubectl plugin for testing
	s.MakeDir(s.ControllerNode(0), "/inttest/bin")
	s.PutFile(s.ControllerNode(0), "/inttest/bin/kubectl-foo", pluginContent)

	// Create the kubectl symlink and make plugins executable
	s.Require().NoError(sshConn.Exec(s.Context(), `
		mkdir /inttest/symlink &&
		ln -s /usr/local/bin/k0s /inttest/symlink/kubectl &&
		chmod +x /inttest/bin/kubectl-*
	`, common.SSHStreams{}))

	s.Require().NoError(s.InitController(0))

	s.T().Log("Check that different ways to call kubectl subcommand work")

	callingConventions := []struct {
		name    string
		cmdline string
	}{
		{"full_subcommand", "/usr/local/bin/k0s kubectl"},
		{"short_subcommand", "/usr/local/bin/k0s kc"},
		{"symlink", "/inttest/symlink/kubectl"},
	}

	subcommands := []struct {
		name    string
		cmdline string
		check   func(t *testing.T, stdout, stderr string, err error)
	}{
		{"plain", "%s", func(t *testing.T, stdout, stderr string, err error) {
			assert.NoError(t, err)
			assert.Contains(t, stdout, "kubectl controls the Kubernetes cluster manager.\n")
			assert.Empty(t, stderr)
		}},

		{"JSON_version", "%s version --output=json", func(t *testing.T, stdout, stderr string, err error) {
			assert.NoError(t, err)
			var versions map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &versions))
			checkClientVersion(t, requiredValue[map[string]any](t, versions, "clientVersion"))
			checkServerVersion(t, requiredValue[map[string]any](t, versions, "serverVersion"))
			assert.Empty(t, stderr)
		}},

		{"plugin_list", "%s plugin list --name-only", func(t *testing.T, stdout, stderr string, err error) {
			assert.NoError(t, err)
			assert.Equal(t, "The following compatible plugins are available:\n\nkubectl-foo\n", stdout)
			assert.Empty(t, stderr)
		}},

		{"plugin_loader_foo", "%s foo", func(t *testing.T, stdout, stderr string, err error) {
			assert.NoError(t, err)
			assert.Equal(t, "kubectl-foo 0\n", stdout)
			assert.Empty(t, stderr)
		}},
		{"plugin_loader_foo_bar", "%s foo bar", func(t *testing.T, stdout, stderr string, err error) {
			assert.NoError(t, err)
			assert.Equal(t, "kubectl-foo 1 bar\n", stdout)
			assert.Empty(t, stderr)
		}},
		{"plugin_loader_foo_bar_arg", "%s foo --bar", func(t *testing.T, stdout, stderr string, err error) {
			assert.NoError(t, err)
			assert.Equal(t, "kubectl-foo 1 --bar\n", stdout)
			assert.Empty(t, stderr)
		}},
		{"plugin_loader_bar_arg_foo", "%s --bar foo", func(t *testing.T, stdout, stderr string, err error) {
			var exitErr *ssh.ExitError
			if assert.ErrorAs(t, err, &exitErr, "Error doesn't have an exit code") {
				assert.Equal(t, 1, exitErr.ExitStatus(), "Exit code mismatch")
			}
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: unknown flag: --bar\nSee 'k0s kubectl --help' for usage.\n", stderr)
		}},

		// This test executes without having kubectl in PATH
		{"plugin_loader_symlink_warning", "PATH=/inttest/bin %s foo", func(t *testing.T, stdout, stderr string, err error) {
			assert.NoError(t, err)
			assert.Equal(t, "kubectl-foo 0\n", stdout)
			assert.Contains(t, stderr, "You can use k0s as a drop-in replacement")
		}},
	}

	for _, callingConvention := range callingConventions {
		for _, subcommand := range subcommands {
			s.T().Run(fmt.Sprint(callingConvention.name, "_", subcommand.name), func(t *testing.T) {
				cmd := fmt.Sprintf(subcommand.cmdline, callingConvention.cmdline)
				cmd = fmt.Sprintf("PATH=/inttest/bin:/inttest/symlink %s", cmd)
				t.Log("Executing", cmd)

				var stdoutBuf bytes.Buffer
				var stderrBuf bytes.Buffer
				err := sshConn.Exec(s.Context(), cmd, common.SSHStreams{Out: &stdoutBuf, Err: &stderrBuf})
				stdout, stderr := stdoutBuf.String(), stderrBuf.String()
				t.Cleanup(func() {
					if t.Failed() {
						t.Log("Error: ", err)
						t.Log("Stdout: ", stdout)
						t.Log("Stderr: ", stderr)
					}
				})

				subcommand.check(t, stdout, stderr, err)
			})
		}
	}
}

func requiredValue[V any](t *testing.T, obj map[string]any, key string) V {
	value, ok := obj[key]
	require.True(t, ok, "Key %q not found", key)
	typedValue, ok := value.(V)
	require.True(t, ok, "Incompatible type for key %q: %+v", key, value)
	return typedValue
}

func checkClientVersion(t *testing.T, v map[string]any) {
	assert.Equal(t,
		constant.KubernetesMajorMinorVersion,
		fmt.Sprintf("%v.%v", requiredValue[string](t, v, "major"), requiredValue[string](t, v, "minor")),
	)
	assert.Contains(t,
		requiredValue[string](t, v, "gitVersion"),
		fmt.Sprintf("v%s", constant.KubernetesMajorMinorVersion),
	)
	assert.Equal(t, "not_available", requiredValue[string](t, v, "gitCommit"))
	assert.Empty(t, requiredValue[string](t, v, "gitTreeState"))
}

func checkServerVersion(t *testing.T, v map[string]any) {
	assert.Equal(t,
		constant.KubernetesMajorMinorVersion,
		fmt.Sprintf("%v.%v", requiredValue[string](t, v, "major"), requiredValue[string](t, v, "minor")),
	)
	assert.Contains(t,
		requiredValue[string](t, v, "gitVersion"),
		fmt.Sprintf("v%s", constant.KubernetesMajorMinorVersion),
	)
	assert.Contains(t, requiredValue[string](t, v, "gitVersion"), "+k0s")
	assert.NotEmpty(t, requiredValue[string](t, v, "gitCommit"))
	assert.Equal(t, "clean", requiredValue[string](t, v, "gitTreeState"))
}

func TestKubectlCommand(t *testing.T) {
	suite.Run(t, &KubectlSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	})
}
