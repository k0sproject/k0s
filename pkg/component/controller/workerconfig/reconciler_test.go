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

package workerconfig

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"

	k8stesting "k8s.io/client-go/testing"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

type kubeletConfig = kubeletv1beta1.KubeletConfiguration

// TODO: simplify it somehow, it is hard to read and to modify, both tests and implementation
func TestReconciler_Lifecycle(t *testing.T) {
	createdReconciler := func(t *testing.T) (*Reconciler, testutil.FakeClientFactory) {
		t.Helper()
		clients := testutil.NewFakeClientFactory()
		k0sVars, err := config.NewCfgVars(nil, t.TempDir())
		require.NoError(t, err)
		underTest, err := NewReconciler(
			k0sVars,
			&v1beta1.ClusterSpec{
				API: &v1beta1.APISpec{},
				Network: &v1beta1.Network{
					ClusterDomain: "test.local",
					ServiceCIDR:   "99.99.99.0/24",
				},
			},
			clients,
			&leaderelector.Dummy{Leader: true},
			true,
		)
		require.NoError(t, err)
		underTest.log = newTestLogger(t)
		return underTest, clients
	}

	t.Run("when_created", func(t *testing.T) {

		t.Run("init_succeeds", func(t *testing.T) {
			underTest, _ := createdReconciler(t)

			err := underTest.Init(testContext(t))

			assert.NoError(t, err)
		})

		t.Run("start_fails", func(t *testing.T) {
			underTest, _ := createdReconciler(t)

			err := underTest.Start(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot start, not initialized: created", err.Error())
		})

		t.Run("reconcile_fails", func(t *testing.T) {
			underTest, _ := createdReconciler(t)

			err := underTest.Reconcile(testContext(t), v1beta1.DefaultClusterConfig(nil))

			require.Error(t, err)
			assert.Equal(t, "cannot reconcile, not started: created", err.Error())
		})

		t.Run("stop_fails", func(t *testing.T) {
			underTest, _ := createdReconciler(t)

			err := underTest.Stop()

			require.Error(t, err)
			assert.Equal(t, "cannot stop: created", err.Error())
		})
	})

	initializedReconciler := func(t *testing.T) (*Reconciler, testutil.FakeClientFactory) {
		t.Helper()
		underTest, clients := createdReconciler(t)
		require.NoError(t, underTest.Init(testContext(t)))
		return underTest, clients
	}

	t.Run("when_initialized", func(t *testing.T) {

		t.Run("another_init_fails", func(t *testing.T) {
			underTest, _ := initializedReconciler(t)

			err := underTest.Init(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot initialize, not created: initialized", err.Error())
		})

		t.Run("start_and_stop_succeed", func(t *testing.T) {
			underTest, _ := initializedReconciler(t)

			require.NoError(t, underTest.Start(testContext(t)))
			assert.NoError(t, underTest.Stop())
		})

		t.Run("reconcile_fails", func(t *testing.T) {
			underTest, _ := initializedReconciler(t)

			err := underTest.Reconcile(testContext(t), v1beta1.DefaultClusterConfig(nil))

			require.Error(t, err)
			assert.Equal(t, "cannot reconcile, not started: initialized", err.Error())
		})

		t.Run("stop_fails", func(t *testing.T) {
			underTest, _ := initializedReconciler(t)

			err := underTest.Stop()

			require.Error(t, err)
			assert.Equal(t, "cannot stop: initialized", err.Error())
		})
	})

	startedReconciler := func(t *testing.T) (*Reconciler, *mockApplier) {
		t.Helper()
		underTest, clients := initializedReconciler(t)
		mockKubernetesEndpoints(t, clients)
		mockApplier := installMockApplier(t, underTest)
		require.NoError(t, underTest.Start(testContext(t)))
		t.Cleanup(func() {
			err := underTest.Stop()
			if !t.Failed() {
				assert.NoError(t, err)
			}
		})
		return underTest, mockApplier
	}

	t.Run("when_started", func(t *testing.T) {

		t.Run("init_fails", func(t *testing.T) {
			underTest, _ := startedReconciler(t)

			err := underTest.Init(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot initialize, not created: started", err.Error())
		})

		t.Run("another_start_fails", func(t *testing.T) {
			underTest, _ := startedReconciler(t)

			err := underTest.Start(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot start, not initialized: started", err.Error())
		})

		t.Run("reconcile_succeeds", func(t *testing.T) {
			underTest, mockApplier := startedReconciler(t)
			applied := mockApplier.expectApply(t, nil)

			assert.NoError(t, underTest.Reconcile(testContext(t), v1beta1.DefaultClusterConfig(nil)))

			assert.NotEmpty(t, applied(), "Expected some resources to be applied")
		})

		t.Run("stop_succeeds", func(t *testing.T) {
			underTest, _ := startedReconciler(t)

			err := underTest.Stop()

			assert.NoError(t, err)
		})
	})

	reconciledReconciler := func(t *testing.T) (*Reconciler, *mockApplier) {
		t.Helper()
		underTest, mockApplier := startedReconciler(t)
		applied := mockApplier.expectApply(t, nil)
		require.NoError(t, underTest.Reconcile(testContext(t), v1beta1.DefaultClusterConfig(nil)))

		_ = applied() // wait until reconciliation happened
		return underTest, mockApplier
	}

	t.Run("when_reconciled", func(t *testing.T) {
		t.Run("init_fails", func(t *testing.T) {
			underTest, _ := reconciledReconciler(t)

			err := underTest.Init(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot initialize, not created: started", err.Error())
		})

		t.Run("start_fails", func(t *testing.T) {
			underTest, _ := reconciledReconciler(t)

			err := underTest.Start(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot start, not initialized: started", err.Error())
		})

		t.Run("another_reconcile_succeeds", func(t *testing.T) {
			underTest, mockApplier := reconciledReconciler(t)
			applied := mockApplier.expectApply(t, nil)
			clusterConfig := v1beta1.DefaultClusterConfig(nil)
			clusterConfig.Spec.WorkerProfiles = v1beta1.WorkerProfiles{
				{Name: "foo", Config: json.RawMessage("{}")},
			}

			assert.NoError(t, underTest.Reconcile(testContext(t), clusterConfig))

			assert.NotEmpty(t, applied(), "Expected some resources to be applied")
		})

		t.Run("stop_succeeds", func(t *testing.T) {
			underTest, _ := reconciledReconciler(t)

			err := underTest.Stop()

			assert.NoError(t, err)
		})
	})

	stoppedReconciler := func(t *testing.T) *Reconciler {
		t.Helper()
		underTest, _ := reconciledReconciler(t)
		require.NoError(t, underTest.Stop())
		return underTest
	}

	t.Run("when_stopped", func(t *testing.T) {

		t.Run("init_fails", func(t *testing.T) {
			underTest := stoppedReconciler(t)

			err := underTest.Init(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot initialize, not created: stopped", err.Error())
		})

		t.Run("start_fails", func(t *testing.T) {
			underTest := stoppedReconciler(t)

			err := underTest.Start(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot start, not initialized: stopped", err.Error())
		})

		t.Run("reconcile_fails", func(t *testing.T) {
			underTest := stoppedReconciler(t)

			err := underTest.Reconcile(testContext(t), v1beta1.DefaultClusterConfig(nil))

			require.Error(t, err)
			assert.Equal(t, "cannot reconcile, not started: stopped", err.Error())
		})

		t.Run("stop_succeeds", func(t *testing.T) {
			underTest := stoppedReconciler(t)

			err := underTest.Stop()

			assert.NoError(t, err)
		})
	})
}

func TestReconciler_ResourceGeneration(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	underTest, err := NewReconciler(
		k0sVars,
		&v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{},
			Network: &v1beta1.Network{
				ClusterDomain: "test.local",
				ServiceCIDR:   "99.99.99.0/24",
			},
		},
		clients,
		&leaderelector.Dummy{Leader: true},
		true,
	)
	require.NoError(t, err)
	underTest.log = newTestLogger(t)

	require.NoError(t, underTest.Init(context.TODO()))

	mockKubernetesEndpoints(t, clients)
	mockApplier := installMockApplier(t, underTest)

	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() {
		assert.NoError(t, underTest.Stop())
	})

	applied := mockApplier.expectApply(t, nil)

	require.NoError(t, underTest.Reconcile(context.TODO(), &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			FeatureGates: v1beta1.FeatureGates{
				v1beta1.FeatureGate{
					Name:       "kubelet-feature",
					Enabled:    true,
					Components: []string{"kubelet"},
				},
			},
			Network: &v1beta1.Network{
				NodeLocalLoadBalancing: &v1beta1.NodeLocalLoadBalancing{
					Enabled: true,
					Type:    v1beta1.NllbTypeEnvoyProxy,
					EnvoyProxy: &v1beta1.EnvoyProxy{
						Image: &v1beta1.ImageSpec{
							Image: "envoy", Version: "test",
						},
						APIServerBindPort: 1337,
					},
				},
			},
			Images: &v1beta1.ClusterImages{
				DefaultPullPolicy: string(corev1.PullNever),
			},
			WorkerProfiles: v1beta1.WorkerProfiles{{
				Name:   "profile_XXX",
				Config: []byte(`{"authentication": {"anonymous": {"enabled": true}}}`),
			}, {
				Name:   "profile_YYY",
				Config: []byte(`{"authentication": {"webhook": {"cacheTTL": "15s"}}}`),
			}},
		},
	}))

	configMaps := map[string]func(t *testing.T, expected *kubeletConfig){
		"worker-config-default-1.27": func(t *testing.T, expected *kubeletConfig) {
			expected.CgroupsPerQOS = pointer.Bool(true)
			expected.FeatureGates = map[string]bool{"kubelet-feature": true}
		},

		"worker-config-default-windows-1.27": func(t *testing.T, expected *kubeletConfig) {
			expected.CgroupsPerQOS = pointer.Bool(false)
			expected.FeatureGates = map[string]bool{"kubelet-feature": true}
		},

		"worker-config-profile_XXX-1.27": func(t *testing.T, expected *kubeletConfig) {
			expected.Authentication.Anonymous.Enabled = pointer.Bool(true)
			expected.FeatureGates = map[string]bool{"kubelet-feature": true}
		},

		"worker-config-profile_YYY-1.27": func(t *testing.T, expected *kubeletConfig) {
			expected.Authentication.Webhook.CacheTTL = metav1.Duration{Duration: 15 * time.Second}
			expected.FeatureGates = map[string]bool{"kubelet-feature": true}
		},
	}

	appliedResources := applied()
	assert.Len(t, appliedResources, len(configMaps)+2)

	for name, mod := range configMaps {
		t.Run(name, func(t *testing.T) {
			kubelet := requireKubelet(t, appliedResources, name)
			expected := makeKubeletConfig(t, func(expected *kubeletConfig) { mod(t, expected) })
			assert.JSONEq(t, expected, kubelet)
		})
	}

	const rbacName = "system:bootstrappers:worker-config"

	t.Run("Role", func(t *testing.T) {
		role := findResource(t, "Expected to find a Role named "+rbacName,
			appliedResources, func(resource *unstructured.Unstructured) bool {
				return resource.GetKind() == "Role" && resource.GetName() == rbacName
			},
		)

		rules, ok, err := unstructured.NestedSlice(role.Object, "rules")
		require.NoError(t, err)
		require.True(t, ok, "No rules field")
		require.Len(t, rules, 1, "Expected a single rule")

		rule, ok := rules[0].(map[string]any)
		require.True(t, ok, "Invalid rule")

		resourceNames, ok, err := unstructured.NestedStringSlice(rule, "resourceNames")
		require.NoError(t, err)
		require.True(t, ok, "No resourceNames field")

		assert.Len(t, resourceNames, len(configMaps))
		for expected := range configMaps {
			assert.Contains(t, resourceNames, expected)
		}
	})

	t.Run("RoleBinding", func(t *testing.T) {
		binding := findResource(t, "Expected to find a RoleBinding named "+rbacName,
			appliedResources, func(resource *unstructured.Unstructured) bool {
				return resource.GetKind() == "RoleBinding" && resource.GetName() == rbacName
			},
		)

		roleRef, ok, err := unstructured.NestedMap(binding.Object, "roleRef")
		if assert.NoError(t, err) && assert.True(t, ok, "No roleRef field") {
			expected := map[string]any{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Role",
				"name":     rbacName,
			}

			assert.Equal(t, expected, roleRef)
		}

		subjects, ok, err := unstructured.NestedSlice(binding.Object, "subjects")
		if assert.NoError(t, err) && assert.True(t, ok, "No subjects field") {
			expected := []any{map[string]any{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Group",
				"name":     "system:bootstrappers",
			}, map[string]any{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Group",
				"name":     "system:nodes",
			}}

			assert.Equal(t, expected, subjects)
		}
	})
}

func TestReconciler_ReconcilesOnChangesOnly(t *testing.T) {
	cluster := v1beta1.DefaultClusterConfig(nil)
	clients := testutil.NewFakeClientFactory()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	underTest, err := NewReconciler(
		k0sVars,
		&v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{},
			Network: &v1beta1.Network{
				ClusterDomain: "test.local",
				ServiceCIDR:   "99.99.99.0/24",
			},
		},
		clients,
		&leaderelector.Dummy{Leader: true},
		true,
	)
	require.NoError(t, err)
	underTest.log = newTestLogger(t)

	require.NoError(t, underTest.Init(context.TODO()))

	mockKubernetesEndpoints(t, clients)
	mockApplier := installMockApplier(t, underTest)

	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() {
		assert.NoError(t, underTest.Stop())
	})

	expectApply := func(t *testing.T) {
		t.Helper()
		applied := mockApplier.expectApply(t, nil)
		assert.NoError(t, underTest.Reconcile(context.TODO(), cluster))
		appliedResources := applied()
		assert.NotEmpty(t, applied, "Expected some resources to be applied")

		for _, r := range appliedResources {
			pp, found, err := unstructured.NestedString(r.Object, "data", "defaultImagePullPolicy")
			assert.NoError(t, err)
			if found {
				t.Logf("%s/%s: %s", r.GetKind(), r.GetName(), pp)
			}
		}
	}

	expectCached := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, underTest.Reconcile(context.TODO(), cluster))
	}

	expectApplyButFail := func(t *testing.T) {
		t.Helper()
		applied := mockApplier.expectApply(t, assert.AnError)
		assert.ErrorIs(t, underTest.Reconcile(context.TODO(), cluster), assert.AnError)
		assert.NotEmpty(t, applied(), "Expected some resources to be applied")
	}

	// Set some value that affects worker configs.
	cluster.Spec.WorkerProfiles = v1beta1.WorkerProfiles{{Name: "foo", Config: json.RawMessage(`{"nodeLeaseDurationSeconds": 1}`)}}
	t.Run("first_time_apply", expectApply)
	t.Run("second_time_cached", expectCached)

	// Change that value, so that configs need to be reapplied.
	cluster.Spec.WorkerProfiles = v1beta1.WorkerProfiles{{Name: "foo", Config: json.RawMessage(`{"nodeLeaseDurationSeconds": 2}`)}}
	t.Run("third_time_apply_fails", expectApplyButFail)

	// After an error, expect a reapplication in any case.
	t.Run("fourth_time_apply", expectApply)

	// Even if the last successfully applied config is restored, expect it to be applied after a failure.
	cluster.Spec.WorkerProfiles = v1beta1.WorkerProfiles{{Name: "foo", Config: json.RawMessage(`{"nodeLeaseDurationSeconds": 1}`)}}
	t.Run("fifth_time_apply", expectApply)
	t.Run("sixth_time_cached", expectCached)
}

func TestReconciler_runReconcileLoop(t *testing.T) {
	underTest := Reconciler{
		log:           newTestLogger(t),
		leaderElector: &leaderelector.Dummy{Leader: true},
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)

	// Prepare update channel for two updates.
	updates, firstDone, secondDone := make(chan updateFunc, 2), make(chan error, 1), make(chan error, 1)

	// Put in the first update.
	updates <- func(s *snapshot) chan<- error { return firstDone }

	// Put in the second update that'll cancel the context.
	updates <- func(s *snapshot) chan<- error { cancel(); return secondDone }

	underTest.runReconcileLoop(ctx, updates, nil)

	switch ctx.Err() {
	case context.Canceled:
		break // this is the good case
	case context.DeadlineExceeded:
		assert.Fail(t, "Test timed out")
	case nil:
		assert.Fail(t, "Loop exited even if the context isn't done")
	default:
		assert.Fail(t, "Unexpected context error", ctx.Err())
	}

	if assert.Len(t, firstDone, 1, "First done channel should contain exactly one element") {
		err, ok := <-firstDone
		assert.True(t, ok)
		assert.NoError(t, err, "Error sent to first done channel when none was expected")
	}

	select {
	case _, ok := <-firstDone:
		assert.False(t, ok, "More than one element sent to first done channel")
	default:
		assert.Fail(t, "First done channel is still open")
	}

	if assert.Len(t, secondDone, 1, "Second done channel should contain exactly one element") {
		err, ok := <-secondDone
		assert.True(t, ok)
		if assert.ErrorIs(t, err, errStoppedConcurrently, "Second done channel didn't indicate concurrent stopping") {
			assert.Equal(t, "stopped concurrently while processing reconciliation", err.Error())
		}
	}
}

func TestReconciler_LeaderElection(t *testing.T) {
	var le mockLeaderElector
	cluster := v1beta1.DefaultClusterConfig(nil)
	clients := testutil.NewFakeClientFactory()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	underTest, err := NewReconciler(
		k0sVars,
		&v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{},
			Network: &v1beta1.Network{
				ClusterDomain: "test.local",
				ServiceCIDR:   "99.99.99.0/24",
			},
		},
		clients,
		&le,
		true,
	)
	require.NoError(t, err)

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	var logs test.Hook
	log.Hooks.Add(&logs)
	underTest.log = log.WithField("test", t.Name())

	require.NoError(t, underTest.Init(context.TODO()))

	mockKubernetesEndpoints(t, clients)
	mockApplier := installMockApplier(t, underTest)

	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() {
		assert.NoError(t, underTest.Stop())
	})

	// Nothing should be applied here, since the leader lease is not acquired.
	assert.NoError(t, underTest.Reconcile(context.TODO(), cluster))

	// Need to wait for two skipped reconciliations: one for the cluster config
	// reconciliation, and another one for the API servers.
	assert.Eventually(t,
		func() bool {
			var numSkips uint
			entries := logs.AllEntries()
			for _, entry := range entries {
				if strings.HasPrefix(entry.Message, "Skipping reconciliation") {
					numSkips++
				}
			}

			switch numSkips {
			case 0, 1:
				return false
			case 2:
				return true
			default:
				require.Fail(t, "Reconciliation skipped too often")
				return true // diverges above
			}
		},
		3*time.Second, 3*time.Millisecond,
		"Expected to observe exactly two skipped reconciliations",
	)

	// Activate the leader lease and expect the resources to be applied.
	applied := mockApplier.expectApply(t, nil)
	le.activate()
	assert.NotEmpty(t, applied(), "Expected some resources to be applied")

	// Deactivate the lease in order to reactivate it a second time.
	le.deactivate()

	// Reactivate the lease and expect another reconciliation, even if the config didn't change.
	applied = mockApplier.expectApply(t, nil)
	le.activate()
	assert.NotEmpty(t, applied(), "Expected some resources to be applied")
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.TODO())
	timeout := time.AfterFunc(10*time.Second, func() {
		assert.Fail(t, "Test context timed out after 10 seconds")
		cancel()
	})
	t.Cleanup(func() {
		timeout.Stop()
		cancel()
	})
	return ctx
}

func newTestLogger(t *testing.T) logrus.FieldLogger {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	return log.WithField("test", t.Name())
}

func requireKubelet(t *testing.T, resources []*unstructured.Unstructured, name string) string {
	configMap := findResource(t, "No ConfigMap found with name "+name,
		resources, func(resource *unstructured.Unstructured) bool {
			return resource.GetKind() == "ConfigMap" && resource.GetName() == name
		},
	)
	kubeletConfigYAML, ok, err := unstructured.NestedString(configMap.Object, "data", "kubeletConfiguration")
	require.NoError(t, err)
	require.True(t, ok, "No data.kubeletConfiguration field")
	kubeletConfigJSON, err := yaml.YAMLToJSONStrict([]byte(kubeletConfigYAML))
	require.NoError(t, err)
	return string(kubeletConfigJSON)
}

func findResource(t *testing.T, failureMessage string, resources resources, probe func(*unstructured.Unstructured) bool) *unstructured.Unstructured {
	for _, resource := range resources {
		if probe(resource) {
			return resource
		}
	}
	require.Fail(t, failureMessage)
	return nil
}

func makeKubeletConfig(t *testing.T, mods ...func(*kubeletConfig)) string {
	kubeletConfig := kubeletConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubeletv1beta1.SchemeGroupVersion.String(),
			Kind:       "KubeletConfiguration",
		},
		ClusterDNS:         []string{"99.99.99.10"},
		ClusterDomain:      "test.local",
		EventRecordQPS:     pointer.Int32(0),
		FailSwapOn:         pointer.Bool(false),
		RotateCertificates: true,
		ServerTLSBootstrap: true,
		TLSMinVersion:      "VersionTLS12",
		TLSCipherSuites: []string{
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
		},
	}

	for _, mod := range mods {
		mod(&kubeletConfig)
	}

	json, err := json.Marshal(&kubeletConfig)
	require.NoError(t, err)
	return string(json)
}

type mockApplier struct {
	ptr atomic.Pointer[[]mockApplierCall]
}

type mockApplierCall = func(resources) error

func (m *mockApplier) expectApply(t *testing.T, retval error) func() resources {
	ch := make(chan resources, 1)

	recv := func() resources {
		select {
		case lastCalled, ok := <-ch:
			if !ok {
				require.Fail(t, "Channel closed unexpectedly")
			}
			return lastCalled

		case <-time.After(10 * time.Second):
			require.Fail(t, "Timed out while waiting for call to apply()")
			return nil // function diverges above
		}
	}

	send := func(r resources) error {
		defer close(ch)
		ch <- r
		return retval
	}

	for {
		calls := m.ptr.Load()
		len := len(*calls)
		newCalls := make([]mockApplierCall, len+1)
		copy(newCalls, *calls)
		newCalls[len] = send
		if m.ptr.CompareAndSwap(calls, &newCalls) {
			break
		}
	}

	return recv
}

func installMockApplier(t *testing.T, underTest *Reconciler) *mockApplier {
	t.Helper()
	mockApplier := mockApplier{}
	mockApplier.ptr.Store(new([]mockApplierCall))

	underTest.mu.Lock()
	defer underTest.mu.Unlock()

	require.Equal(t, reconcilerInitialized, underTest.state, "unexpected state")
	require.NotNil(t, underTest.apply)
	t.Cleanup(func() {
		for _, call := range *mockApplier.ptr.Swap(nil) {
			assert.NoError(t, call(nil))
		}
	})

	underTest.apply = func(ctx context.Context, r resources) error {
		if r == nil {
			panic("cannot call apply() with nil resources")
		}

		for {
			expected := mockApplier.ptr.Load()
			if len(*expected) < 1 {
				panic("unexpected call to apply")
			}

			newExpected := (*expected)[1:]
			if mockApplier.ptr.CompareAndSwap(expected, &newExpected) {
				return (*expected)[0](r)
			}
		}
	}

	return &mockApplier
}

func mockKubernetesEndpoints(t *testing.T, clients testutil.FakeClientFactory) {
	t.Helper()
	client, err := clients.GetClient()
	require.NoError(t, err)

	ep := corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{ResourceVersion: t.Name()},
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{
				{IP: "127.10.10.1"},
			},
			Ports: []corev1.EndpointPort{
				{Name: "https", Port: 6443, Protocol: corev1.ProtocolTCP},
			},
		}},
	}

	epList := corev1.EndpointsList{
		ListMeta: metav1.ListMeta{ResourceVersion: t.Name()},
		Items:    []corev1.Endpoints{ep},
	}

	_, err = client.CoreV1().Endpoints("default").Create(context.TODO(), ep.DeepCopy(), metav1.CreateOptions{})
	require.NoError(t, err)

	clients.Client.CoreV1().(*fake.FakeCoreV1).PrependReactor("list", "endpoints", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, epList.DeepCopy(), nil
	})
}

type mockLeaderElector struct {
	mu       sync.Mutex
	leader   bool
	acquired []func()
}

func (e *mockLeaderElector) activate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.leader {
		e.leader = true
		for _, fn := range e.acquired {
			fn()
		}
	}
}

func (e *mockLeaderElector) deactivate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.leader = false
}

func (e *mockLeaderElector) IsLeader() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.leader
}

func (e *mockLeaderElector) AddAcquiredLeaseCallback(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.acquired = append(e.acquired, fn)
	if e.leader {
		fn()
	}
}

func (e *mockLeaderElector) AddLostLeaseCallback(func()) {
	panic("not expected to be called in tests")
}
