// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
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

	adminConfPath := filepath.Join(dataDir, "admin.conf")
	require.NoError(t, clientcmd.WriteToFile(api.Config{
		Clusters: map[string]*api.Cluster{
			t.Name(): {Server: "https://localhost:65432"},
		},
	}, adminConfPath))

	k0sVars := &config.CfgVars{
		AdminKubeConfigPath: adminConfPath,
		DataDir:             dataDir,
		RuntimeConfigPath:   filepath.Join(dataDir, "run", "k0s.yaml"),
		StartupConfigPath:   configPath,
	}
	require.NoError(t, os.Mkdir(filepath.Dir(k0sVars.RuntimeConfigPath), 0700))
	cfg, err := config.NewRuntimeConfig(k0sVars, nil)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, cfg.Spec.Cleanup()) })

	var stdout bytes.Buffer
	var stderr strings.Builder
	underTest := cmd.NewRootCmd()
	underTest.SetArgs([]string{"kubeconfig", "--data-dir", dataDir, "admin"})
	underTest.SetOut(&stdout)
	underTest.SetErr(&stderr)

	assert.NoError(t, underTest.Execute())

	assert.Empty(t, stderr.String())

	adminConf, err := clientcmd.Load(stdout.Bytes())
	require.NoError(t, err)

	if theCluster, ok := adminConf.Clusters[t.Name()]; assert.True(t, ok) {
		assert.Equal(t, "https://not-here.example.com:65432", theCluster.Server)
	}
}

func TestAdmin_NoAdminConfig(t *testing.T) {
	dataDir := t.TempDir()

	configPath := filepath.Join(dataDir, "k0s.yaml")
	writeYAML(t, configPath, &v1beta1.ClusterConfig{
		TypeMeta: metav1.TypeMeta{APIVersion: v1beta1.SchemeGroupVersion.String(), Kind: v1beta1.ClusterConfigKind},
		Spec: &v1beta1.ClusterSpec{API: &v1beta1.APISpec{
			Port: 65432, ExternalAddress: "not-here.example.com",
		}},
	})

	k0sVars := &config.CfgVars{
		AdminKubeConfigPath: filepath.Join(dataDir, "admin.conf"),
		DataDir:             dataDir,
		RuntimeConfigPath:   filepath.Join(dataDir, "run", "k0s.yaml"),
		StartupConfigPath:   filepath.Join(dataDir, "k0s.yaml"),
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
	msg := fmt.Sprintf("admin config %q not found, check if the control plane is initialized on this node", k0sVars.AdminKubeConfigPath)
	assert.Equal(t, "Error: "+msg+"\n", stderr.String())
}

func writeYAML(t *testing.T, path string, data any) {
	bytes, err := yaml.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, bytes, 0644))
}
