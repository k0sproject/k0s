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
	"io"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KubectlSuite struct {
	common.FootlooseSuite
}

const pluginContent = `#!/usr/bin/env sh

echo foo-plugin
`

func (s *KubectlSuite) TestEmbeddedKubectl() {
	s.Require().NoError(s.InitController(0))
	s.PutFile(s.ControllerNode(0), "/bin/kubectl-foo", pluginContent)

	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err, "failed to SSH into controller")
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(s.Context(), "chmod +x /bin/kubectl-foo")
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "ln -s /usr/local/bin/k0s /usr/bin/kubectl")
	s.Require().NoError(err)

	s.T().Log("Check that different ways to call kubectl subcommand work")

	callingConventions := []struct {
		name string
		cmd  []string
	}{
		{"full_subcommand", []string{"/usr/local/bin/k0s", "kubectl"}},
		{"short_subcommand", []string{"/usr/local/bin/k0s", "kc"}},
		{"symlink", []string{"/usr/bin/kubectl"}},
	}

	subcommands := []struct {
		name  string
		cmd   []string
		check func(t *testing.T, output string)
	}{
		{"plain", nil, func(t *testing.T, output string) {
			assert.Contains(t, output, `kubectl controls the Kubernetes cluster manager.`)
		}},

		{"JSON_version", []string{"version", "--output=json"}, func(t *testing.T, output string) {
			var versions map[string]any
			require.NoError(t, json.Unmarshal([]byte(output), &versions))
			checkClientVersion(t, requiredValue[map[string]any](t, versions, "clientVersion"))
			checkServerVersion(t, requiredValue[map[string]any](t, versions, "serverVersion"))
		}},

		{"plugin_loader", []string{"foo"}, func(t *testing.T, output string) {
			assert.Equal(t, "foo-plugin", output)
		}},
	}

	for _, callingConvention := range callingConventions {
		for _, subcommand := range subcommands {
			s.T().Run(fmt.Sprint(callingConvention.name, "_", subcommand.name), func(t *testing.T) {
				cmd := strings.Join(append(callingConvention.cmd, subcommand.cmd...), " ")
				t.Log("Executing", cmd)
				output, err := ssh.ExecWithOutput(s.Context(), cmd)
				t.Cleanup(func() {
					if t.Failed() {
						t.Log("Error: ", err)
						t.Log("Output: ", output)
					}
				})
				assert.NoError(t, err)
				subcommand.check(t, output)
			})
		}
	}

	s.T().Run("plugin list", func(t *testing.T) {
		require := require.New(t)

		s.PutFile(s.ControllerNode(0), "/bin/kubectl-testplug", "#!/bin/sh\necho \"testplug called with args: $*\"\n")
		_, err := ssh.ExecWithOutput(s.Context(), "chmod +x /bin/kubectl-testplug")
		require.NoError(err)

		output, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubectl plugin list")
		require.NoError(err)
		require.Contains(output, "kubectl-testplug")
	})

	s.T().Run("plugin arg passing", func(t *testing.T) {
		out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubectl testplug hello")
		s.Require().NoError(err)
		s.Require().Contains(out, "testplug called with args: hello")

		out, err = ssh.ExecWithOutput(s.Context(), "kubectl testplug --help")
		s.Require().NoError(err)
		s.Require().Equal("testplug called with args: --help", out)
	})

	// Try with kubectl symlink, a warning should not be printed
	var errOut bytes.Buffer
	streams := common.SSHStreams{In: nil, Out: io.Discard, Err: &errOut}
	s.Require().NoError(ssh.Exec(s.Context(), "/usr/local/bin/k0s kubectl testplug hello", streams))
	s.Require().NotContains(errOut.String(), "You can use k0s as a drop-in replacement")

	// Try without kubectl symlink, a warning should be printed
	_, err = ssh.ExecWithOutput(s.Context(), "rm -f /usr/bin/kubectl /bin/kubectl /usr/local/bin/kubectl")
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "command -v kubectl")
	s.Require().Error(err)
	errOut.Reset()
	s.Require().NoError(ssh.Exec(s.Context(), "/usr/local/bin/k0s kubectl testplug hello", streams))
	s.Require().Contains(errOut.String(), "You can use k0s as a drop-in replacement")

	// restore link for any other tests
	_, _ = ssh.ExecWithOutput(s.Context(), "ln -s /usr/local/bin/k0s /usr/bin/kubectl")
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
