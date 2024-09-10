/*
Copyright 2021 k0s authors

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

package kubeconfig_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeconfigCreate(t *testing.T) {
	// Create a default cluster config with a special external address
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.API.ExternalAddress = "10.0.0.86"

	// Create some k0sVars
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)

	// Create the CA
	require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0755))
	certManager := certificate.Manager{K0sVars: k0sVars}
	require.NoError(t, certManager.EnsureCA("ca", t.Name()))

	// Setup the kubeconfig command
	configData, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	cmd := cmd.NewRootCmd()
	cmd.SetArgs([]string{
		"--config", "-",
		"--data-dir", k0sVars.DataDir,
		"kubeconfig", "create", "test-user",
	})
	var stdout, stderr bytes.Buffer
	cmd.SetIn(bytes.NewReader(configData))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Try to create a kubeconfig for test-user
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stderr.Bytes())

	// Write kubeconfig to a file in order to load it afterwards
	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(kubeconfigPath, stdout.Bytes(), 0644))

	// Load the kubeconfig from disk and verify that the API server host is right
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)
	assert.Equal(t, "https://10.0.0.86:6443", config.Host)

	// Ensure that the certificates are embedded
	assert.Empty(t, config.CAFile, "Config should be self-contained")
	if data, err := os.ReadFile(filepath.Join(k0sVars.CertRootDir, "ca.crt")); assert.NoError(t, err) {
		assert.Equal(t, data, config.CAData)
	}
	assert.Empty(t, config.CertFile, "Config should be self-contained")
	if data, err := os.ReadFile(filepath.Join(k0sVars.CertRootDir, "test-user.crt")); assert.NoError(t, err) {
		assert.Equal(t, data, config.CertData)
	}
	assert.Empty(t, config.KeyFile, "Config should be self-contained")
	if data, err := os.ReadFile(filepath.Join(k0sVars.CertRootDir, "test-user.key")); assert.NoError(t, err) {
		assert.Equal(t, data, config.KeyData)
	}
}
