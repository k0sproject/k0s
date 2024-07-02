/*
Copyright 2023 k0s authors

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

package config

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func validConfig() []byte {
	config := v1beta1.DefaultClusterConfig()
	cfg, _ := yaml.Marshal(config)
	return cfg
}

func invalidConfig() []byte {
	config := v1beta1.DefaultClusterConfig()
	// NLLB cannot be used with external address
	config.Spec.Network.NodeLocalLoadBalancing.Enabled = true
	config.Spec.API.ExternalAddress = "k0s.example.com"
	cfg, _ := yaml.Marshal(config)
	return cfg
}

func TestValidateCmd(t *testing.T) {
	t.Run("stdin", func(t *testing.T) {
		cmd := NewValidateCmd()
		cmd.SetArgs([]string{"--config", "-"})
		cmd.SetIn(bytes.NewReader(invalidConfig()))
		errOut := bytes.NewBuffer(nil)
		cmd.SetErr(errOut)
		assert.Error(t, cmd.Execute())
		assert.Contains(t, errOut.String(), "node-local load balancing cannot be used in conjunction with an external Kubernetes API server address")
		errOut.Reset()
		cmd.SetIn(bytes.NewReader(validConfig()))
		assert.NoError(t, cmd.Execute())
		assert.Empty(t, errOut.String())
	})

	t.Run("empty config argument", func(t *testing.T) {
		cmd := NewValidateCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--config", ""})
		assert.ErrorContains(t, cmd.Execute(), "empty")
	})

	t.Run("nonexisting config file", func(t *testing.T) {
		cmd := NewValidateCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--config", "/path/to/nonexistent/file"})
		assert.ErrorIs(t, cmd.Execute(), os.ErrNotExist)
	})

	t.Run("malformed config from file", func(t *testing.T) {
		cmd := NewValidateCmd()
		tmpfile, _ := os.CreateTemp("", "testconfig")
		defer os.Remove(tmpfile.Name())
		_, _ = tmpfile.WriteString("malformed yaml")
		cmd.SetArgs([]string{"--config", tmpfile.Name()})
		errOut := bytes.NewBuffer(nil)
		cmd.SetErr(errOut)
		assert.ErrorContains(t, cmd.Execute(), "cannot unmarshal")
		assert.Contains(t, errOut.String(), "cannot unmarshal")
	})

	t.Run("valid config from file", func(t *testing.T) {
		cmd := NewValidateCmd()
		tmpfile, _ := os.CreateTemp("", "testconfig")
		defer os.Remove(tmpfile.Name())
		_, _ = tmpfile.Write(validConfig())
		cmd.SetArgs([]string{"--config", tmpfile.Name()})
		errOut := bytes.NewBuffer(nil)
		cmd.SetErr(errOut)
		assert.NoError(t, cmd.Execute())
		assert.Empty(t, errOut.String())
	})
}
