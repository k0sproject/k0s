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

package applier_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_AppliesStacks(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()

	underTest := applier.Manager{
		K0sVars:           k0sVars,
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}

	// A stack that exists on disk before the manager is started.
	before := filepath.Join(k0sVars.ManifestsDir, "before")
	require.NoError(t, os.MkdirAll(before, constant.ManifestsDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(before, "before.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: before
  namespace: default
  resourceVersion: "1"
data: {}
`,
	), constant.CertMode))

	require.NoError(t, leaderElector.Init(context.TODO()))
	require.NoError(t, underTest.Init(context.TODO()))
	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })

	// Wait for the "before" stack to be applied.
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps("default")).
		Until(context.TODO(), func(item *corev1.ConfigMap) (bool, error) {
			return item.Name == "before", nil
		}),
	)

	// A stack that is created on disk after the manager has started.
	after := filepath.Join(k0sVars.ManifestsDir, "after")
	require.NoError(t, os.MkdirAll(after, constant.ManifestsDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(after, "after.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: after
  namespace: default
  resourceVersion: "1"
data: {}
`,
	), constant.CertMode))

	// Wait for the "after" stack to be applied.
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps("default")).
		Until(context.TODO(), func(item *corev1.ConfigMap) (bool, error) {
			return item.Name == "after", nil
		}),
	)
}

func TestManager_IgnoresStacks(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()

	underTest := applier.Manager{
		K0sVars:           k0sVars,
		IgnoredStacks:     map[string]string{"ignored": "v99"},
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}

	ignored := filepath.Join(k0sVars.ManifestsDir, "ignored")
	require.NoError(t, os.MkdirAll(ignored, constant.ManifestsDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(ignored, "ignored.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: ignored
  namespace: default
data: {}
`,
	), constant.CertMode))

	require.NoError(t, leaderElector.Init(context.TODO()))
	require.NoError(t, underTest.Init(context.TODO()))
	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })

	var content []byte
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		content, err = os.ReadFile(filepath.Join(ignored, "ignored.txt"))
		assert.NoError(t, err)
	}, 10*time.Second, 100*time.Millisecond)

	expectedContent := []string{
		"The ignored stack is handled internally since k0s v99.",
		"This directory is ignored and can be safely removed.",
		"",
	}
	assert.Equal(t, strings.Join(expectedContent, "\n"), string(content))

	configMaps, err := clients.Client.CoreV1().ConfigMaps("default").List(context.TODO(), metav1.ListOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, configMaps.Items)
	}
}
