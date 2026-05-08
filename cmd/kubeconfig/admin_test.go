// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmin(t *testing.T) {
	dataDir := t.TempDir()

	configPath := filepath.Join(dataDir, "k0s.yaml")
	writeYAML(t, configPath, &v1beta1.ClusterConfig{
		TypeMeta: metav1.TypeMeta{APIVersion: v1beta1.SchemeGroupVersion.String(), Kind: v1beta1.ClusterConfigKind},
		Spec: &v1beta1.ClusterSpec{API: &v1beta1.APISpec{
			Port: 65432, ExternalAddress: "not-here.example.com",
		}},
	})

	certRootDir := filepath.Join(dataDir, "pki")
	require.NoError(t, os.Mkdir(certRootDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(certRootDir, "ca.crt"), []byte("contents of ca.crt"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(certRootDir, "admin.crt"), []byte("contents of admin.crt"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(certRootDir, "admin.key"), []byte("contents of admin.key"), 0600))

	k0sVars := &config.CfgVars{
		StartupConfigPath: configPath,
		RuntimeConfigPath: filepath.Join(dataDir, "run", "k0s.yaml"),
		DataDir:           dataDir,
		CertRootDir:       certRootDir,
	}
	require.NoError(t, os.Mkdir(filepath.Dir(k0sVars.RuntimeConfigPath), 0700))
	cfg, err := config.NewRuntimeConfig(k0sVars, nil)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, cfg.Spec.Cleanup()) })

	var stdout, stderr strings.Builder
	underTest := cmd.NewRootCmd()
	underTest.SetArgs([]string{"kubeconfig", "--data-dir", dataDir, "admin"})
	underTest.SetOut(&stdout)
	underTest.SetErr(&stderr)

	assert.NoError(t, underTest.Execute())
	assert.Empty(t, stderr.String())
	assert.Equal(t, `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: Y29udGVudHMgb2YgY2EuY3J0` /* contents of ca.crt */ +`
    server: https://not-here.example.com:65432
  name: k0s
contexts:
- context:
    cluster: k0s
    user: admin
  name: k0s-admin
current-context: k0s-admin
kind: Config
users:
- name: admin
  user:
    client-certificate-data: Y29udGVudHMgb2YgYWRtaW4uY3J0` /* contents of admin.crt */ +`
    client-key-data: Y29udGVudHMgb2YgYWRtaW4ua2V5` /* contents of admin.key */ +`
`,
		stdout.String(),
	)
}

func TestAdmin_NoAdminConfig(t *testing.T) {
	dataDir := t.TempDir()

	configPath := filepath.Join(dataDir, "k0s.yaml")
	require.NoError(t, os.WriteFile(configPath, nil, 0644))

	k0sVars := &config.CfgVars{
		StartupConfigPath: configPath,
		RuntimeConfigPath: filepath.Join(dataDir, "run", "k0s.yaml"),
		DataDir:           dataDir,
		CertRootDir:       filepath.Join(dataDir, "pki"),
	}
	require.NoError(t, os.Mkdir(filepath.Dir(k0sVars.RuntimeConfigPath), 0700))

	cfg, err := config.NewRuntimeConfig(k0sVars, nil)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, cfg.Spec.Cleanup()) })

	var stdout, stderr strings.Builder
	underTest := cmd.NewRootCmd()
	underTest.SetArgs([]string{"kubeconfig", "--data-dir", dataDir, "admin"})
	underTest.SetOut(&stdout)
	underTest.SetErr(&stderr)

	assert.Error(t, underTest.Execute())

	assert.Empty(t, stdout.String())
	msg := fmt.Sprintf("admin PKI file %q not found, check if the control plane is initialized on this node", filepath.Join(dataDir, "pki", "admin.crt"))
	assert.Equal(t, "Error: "+msg+"\n", stderr.String())
}

func writeYAML(t *testing.T, path string, data any) {
	bytes, err := yaml.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, bytes, 0644))
}
