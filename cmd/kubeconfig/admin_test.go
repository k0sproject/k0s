/*
Copyright 2024 k0s authors

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

	rtConfigPath := filepath.Join(dataDir, "run", "k0s.yaml")
	writeYAML(t, rtConfigPath, &config.RuntimeConfig{
		TypeMeta: metav1.TypeMeta{APIVersion: v1beta1.SchemeGroupVersion.String(), Kind: config.RuntimeConfigKind},
		Spec: &config.RuntimeConfigSpec{K0sVars: &config.CfgVars{
			AdminKubeConfigPath: adminConfPath,
			DataDir:             dataDir,
			RuntimeConfigPath:   rtConfigPath,
			StartupConfigPath:   configPath,
		}},
	})

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
	adminConfPath := filepath.Join(dataDir, "admin.conf")
	rtConfigPath := filepath.Join(dataDir, "run", "k0s.yaml")
	writeYAML(t, rtConfigPath, &config.RuntimeConfig{
		TypeMeta: metav1.TypeMeta{APIVersion: v1beta1.SchemeGroupVersion.String(), Kind: config.RuntimeConfigKind},
		Spec: &config.RuntimeConfigSpec{K0sVars: &config.CfgVars{
			AdminKubeConfigPath: adminConfPath,
			DataDir:             dataDir,
			RuntimeConfigPath:   rtConfigPath,
			StartupConfigPath:   configPath,
		}},
	})

	var stdout, stderr strings.Builder
	underTest := cmd.NewRootCmd()
	underTest.SetArgs([]string{"kubeconfig", "--data-dir", dataDir, "admin"})
	underTest.SetOut(&stdout)
	underTest.SetErr(&stderr)

	assert.Error(t, underTest.Execute())

	assert.Empty(t, stdout.String())
	msg := fmt.Sprintf("admin config %q not found, check if the control plane is initialized on this node", adminConfPath)
	assert.Equal(t, "Error: "+msg+"\n", stderr.String())
}

func writeYAML(t *testing.T, path string, data any) {
	bytes, err := yaml.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, bytes, 0644))
}
