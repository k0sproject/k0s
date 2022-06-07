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
	"encoding/json"
	"fmt"
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

	ssh, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err, "failed to SSH into controller")
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput("chmod +x /bin/kubectl-foo")
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput("ln -s /usr/bin/k0s /usr/bin/kubectl")
	s.Require().NoError(err)

	s.T().Log("Check that different ways to call kubectl subcommand work")

	callingConventions := []struct {
		name string
		cmd  []string
	}{
		{"full_subcommand", []string{"k0s", "kubectl"}},
		{"short_subcommand", []string{"k0s", "kc"}},
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
				output, err := ssh.ExecWithOutput(cmd)
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
