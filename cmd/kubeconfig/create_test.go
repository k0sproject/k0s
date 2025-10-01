// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	require.NoError(t, certManager.EnsureCA("ca", t.Name(), 87600*time.Hour))

	// Setup the kubeconfig command
	configData, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	cmd := cmd.NewRootCmd()
	cmd.SetArgs([]string{
		"--config", "-",
		"--data-dir", k0sVars.DataDir,
		"kubeconfig", "create", "test-user",
		"--context-name", "my-cluster",
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

	// Verify that the current context is set as requested
	rawConfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	require.NoError(t, err)
	assert.Equal(t, "my-cluster", rawConfig.CurrentContext)

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
