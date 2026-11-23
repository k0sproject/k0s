// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeRouterCNIStatus(t *testing.T) {
	t.Parallel()

	t.Run("healthy", func(t *testing.T) {
		t.Parallel()
		got := kubeRouterCondition(t, kubeRouterDS(1, 1))
		assert.Equal(t, Condition{
			Type:    conditionCNIReady,
			Status:  corev1.ConditionTrue,
			Reason:  reasonAllReady,
			Message: "kube-router DaemonSet reports 1/1 available, 1 updated",
		}, got)
	})

	t.Run("not ready", func(t *testing.T) {
		t.Parallel()
		got := kubeRouterCondition(t, kubeRouterDS(1, 0))
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "kube-router DaemonSet reports 0/1 available, 1 updated", got.Message)
	})

	t.Run("zero desired", func(t *testing.T) {
		t.Parallel()
		got := kubeRouterCondition(t, kubeRouterDS(0, 0))
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonNoScheduledPods, got.Reason)
		assert.Equal(t, "kube-router DaemonSet reports 0/0 available, 0 updated", got.Message)
	})

	t.Run("zero desired beats a stale generation", func(t *testing.T) {
		t.Parallel()
		ds := kubeRouterDS(0, 0)
		ds.Generation = 2
		ds.Status.ObservedGeneration = 1
		got := kubeRouterCondition(t, ds)
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonNoScheduledPods, got.Reason)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		got := kubeRouterCondition(t)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotFound, got.Reason)
		assert.Equal(t, "kube-router DaemonSet not found", got.Message)
	})

	t.Run("API error", func(t *testing.T) {
		t.Parallel()
		factory := testutil.NewFakeClientFactory()
		factory.Client.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver unavailable")
		})
		got := reconciledCNIStatus(t, &Status{ControllerClientFactory: factory}, kubeRouterConfig())
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, "apiserver unavailable", got.Message)
	})

	t.Run("stale generation", func(t *testing.T) {
		t.Parallel()
		ds := kubeRouterDS(1, 1)
		ds.Generation = 2
		ds.Status.ObservedGeneration = 1
		got := kubeRouterCondition(t, ds)
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonStaleGeneration, got.Reason)
		assert.Equal(t, "kube-router DaemonSet has not observed generation 2 (observed 1)", got.Message)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		factory := testutil.NewFakeClientFactory()
		factory.Client.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, context.DeadlineExceeded
		})
		got := reconciledCNIStatus(t, &Status{ControllerClientFactory: factory}, kubeRouterConfig())
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonTimeout, got.Reason)
		assert.Equal(t, "CNI status check timed out", got.Message)
	})

	t.Run("canceled is an error, not a timeout", func(t *testing.T) {
		t.Parallel()
		factory := testutil.NewFakeClientFactory()
		factory.Client.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, context.Canceled
		})
		got := reconciledCNIStatus(t, &Status{ControllerClientFactory: factory}, kubeRouterConfig())
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, context.Canceled.Error(), got.Message)
	})

	t.Run("old pods during rollout", func(t *testing.T) {
		t.Parallel()
		ds := kubeRouterDS(2, 2)
		ds.Status.UpdatedNumberScheduled = 1
		got := kubeRouterCondition(t, ds)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "kube-router DaemonSet reports 2/2 available, 1 updated", got.Message)
	})
}

func TestCalicoCNIStatus(t *testing.T) {
	t.Parallel()

	t.Run("healthy", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(false), calicoNodeDS(2, 2), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.Equal(t, reasonAllReady, got.Reason)
		assert.Equal(t, "calico-node DaemonSet reports 2/2 available, 2 updated; calico-kube-controllers Deployment is available", got.Message)
	})

	t.Run("node not ready", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(false), calicoNodeDS(2, 1), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-node DaemonSet reports 1/2 available, 2 updated", got.Message)
	})

	t.Run("controllers not ready", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(false), calicoNodeDS(1, 1), calicoControllers(corev1.ConditionFalse))
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment is not available", got.Message)
	})

	t.Run("controllers not found", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(false), calicoNodeDS(1, 1))
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotFound, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment not found", got.Message)
	})

	t.Run("controllers API error", func(t *testing.T) {
		t.Parallel()
		factory := testutil.NewFakeClientFactory(calicoNodeDS(1, 1))
		factory.Client.PrependReactor("get", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver unavailable")
		})
		got := reconciledCNIStatus(t, &Status{
			ControllerClientFactory: factory,
			HasWindowsNodes:         windowsCounter(new(false)),
		}, clusterConfigWithCalico(""))
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, "apiserver unavailable", got.Message)
	})

	t.Run("prefers false over unknown", func(t *testing.T) {
		t.Parallel()
		factory := testutil.NewFakeClientFactory(calicoNodeDS(1, 0))
		factory.Client.PrependReactor("get", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver unavailable")
		})
		got := reconciledCNIStatus(t, &Status{
			ControllerClientFactory: factory,
			HasWindowsNodes:         windowsCounter(new(false)),
		}, clusterConfigWithCalico(""))
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-node DaemonSet reports 0/1 available, 1 updated", got.Message)
	})

	t.Run("windows not found is ignored", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(false), calicoNodeDS(1, 1), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.NotContains(t, got.Message, calicoWindowsDaemonSet)
	})

	t.Run("windows not ready", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(true), calicoNodeDS(1, 1), calicoWindowsDS(1, 0), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-node-windows DaemonSet reports 0/1 available, 1 updated", got.Message)
	})

	t.Run("windows ready", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(true), calicoNodeDS(1, 1), calicoWindowsDS(1, 1), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.Equal(t, "calico-node DaemonSet reports 1/1 available, 1 updated; calico-node-windows DaemonSet reports 1/1 available, 1 updated; calico-kube-controllers Deployment is available", got.Message)
	})

	t.Run("controllers unknown", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(false), calicoNodeDS(1, 1), calicoControllers(corev1.ConditionUnknown))
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment availability is unknown", got.Message)
	})

	t.Run("windows required when Windows nodes exist", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", new(true), calicoNodeDS(1, 1), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotFound, got.Reason)
		assert.Equal(t, "calico-node-windows DaemonSet not found", got.Message)
	})

	t.Run("windows count unknown", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, "", nil, calicoNodeDS(1, 1), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonWindowsNodesUnknown, got.Reason)
		assert.Equal(t, "Windows node count is not yet known", got.Message)
	})

	t.Run("BIRD mode does not require the Windows DaemonSet", func(t *testing.T) {
		t.Parallel()
		got := calicoCondition(t, v1beta1.CalicoModeBIRD, new(true), calicoNodeDS(1, 1), calicoControllers(corev1.ConditionTrue))
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.NotContains(t, got.Message, calicoWindowsDaemonSet)
	})

	t.Run("controllers stale generation", func(t *testing.T) {
		t.Parallel()
		deploy := calicoControllers(corev1.ConditionTrue)
		deploy.Generation = 3
		deploy.Status.ObservedGeneration = 2
		got := calicoCondition(t, "", new(false), calicoNodeDS(1, 1), deploy)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment has not observed generation 3 (observed 2)", got.Message)
	})

	t.Run("controllers old replicas during rollout", func(t *testing.T) {
		t.Parallel()
		deploy := calicoControllers(corev1.ConditionTrue)
		deploy.Status.UpdatedReplicas = 0
		deploy.Status.AvailableReplicas = 1
		got := calicoCondition(t, "", new(false), calicoNodeDS(1, 1), deploy)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment reports 0/1 updated, 1 available, 1 total", got.Message)
	})

	t.Run("controllers old pods remain", func(t *testing.T) {
		t.Parallel()
		deploy := calicoControllers(corev1.ConditionTrue)
		deploy.Status.Replicas = 2
		deploy.Status.UpdatedReplicas = 1
		deploy.Status.AvailableReplicas = 1
		got := calicoCondition(t, "", new(false), calicoNodeDS(1, 1), deploy)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment reports 1/1 updated, 1 available, 2 total", got.Message)
	})
}

func TestCNIStatusHandler(t *testing.T) {
	t.Parallel()

	t.Run("skips without a controller client factory", func(t *testing.T) {
		t.Parallel()
		got := fetchStatus(t, &Status{
			StatusInformation: K0sStatus{ClusterConfig: kubeRouterConfig()},
		})
		assert.Empty(t, got.Conditions)
	})

	t.Run("skips custom CNI", func(t *testing.T) {
		t.Parallel()
		st := &Status{ControllerClientFactory: testutil.NewFakeClientFactory()}
		reconcile(t, st, clusterConfigWithProvider("custom"))
		got := fetchStatus(t, st)
		assert.Empty(t, got.Conditions)
	})

	t.Run("skips custom CNI when the controller client fails", func(t *testing.T) {
		t.Parallel()
		st := &Status{ControllerClientFactory: &failingClientFactory{err: errors.New("admin kubeconfig missing")}}
		reconcile(t, st, clusterConfigWithProvider("custom"))
		got := fetchStatus(t, st)
		assert.Empty(t, got.Conditions)
	})

	t.Run("reports an error before the network is reconciled", func(t *testing.T) {
		t.Parallel()
		got := fetchStatus(t, &Status{
			StatusInformation:       K0sStatus{ClusterConfig: kubeRouterConfig()},
			ControllerClientFactory: testutil.NewFakeClientFactory(kubeRouterDS(1, 1)),
		})
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionUnknown, got.Conditions[0].Status)
		assert.Equal(t, reasonError, got.Conditions[0].Reason)
		assert.Equal(t, "network configuration has not been reconciled yet", got.Conditions[0].Message)
	})

	t.Run("controller client error", func(t *testing.T) {
		t.Parallel()
		st := &Status{ControllerClientFactory: &failingClientFactory{err: errors.New("admin kubeconfig missing")}}
		reconcile(t, st, kubeRouterConfig())
		got := fetchStatus(t, st)
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionUnknown, got.Conditions[0].Status)
		assert.Equal(t, reasonError, got.Conditions[0].Reason)
		assert.Equal(t, "admin kubeconfig missing", got.Conditions[0].Message)
	})

	t.Run("controller without workloads still checks CNI", func(t *testing.T) {
		t.Parallel()
		st := &Status{
			StatusInformation:       K0sStatus{Workloads: false},
			ControllerClientFactory: testutil.NewFakeClientFactory(kubeRouterDS(1, 1)),
		}
		reconcile(t, st, kubeRouterConfig())
		got := fetchStatus(t, st)
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionTrue, got.Conditions[0].Status)
		assert.False(t, got.WorkerToAPIConnectionStatus.Success)
	})

	t.Run("reconciled VXLAN mode requires the Windows DaemonSet", func(t *testing.T) {
		t.Parallel()
		st := &Status{
			StatusInformation: K0sStatus{ClusterConfig: clusterConfigWithCalico(v1beta1.CalicoModeBIRD)},
			ControllerClientFactory: testutil.NewFakeClientFactory(
				calicoNodeDS(1, 1),
				calicoControllers(corev1.ConditionTrue),
			),
			HasWindowsNodes: windowsCounter(new(true)),
		}
		reconcile(t, st, clusterConfigWithCalico(v1beta1.CalicoModeBIRD))
		got := fetchStatus(t, st)
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionTrue, got.Conditions[0].Status, "BIRD mode does not require calico-node-windows")

		reconcile(t, st, clusterConfigWithCalico(v1beta1.CalicoModeVXLAN))
		got = fetchStatus(t, st)
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionFalse, got.Conditions[0].Status)
		assert.Equal(t, reasonNotFound, got.Conditions[0].Reason)
		assert.Equal(t, "calico-node-windows DaemonSet not found", got.Conditions[0].Message)
	})

	t.Run("reconcile keeps the node-local Kine config", func(t *testing.T) {
		t.Parallel()
		got := statusAfterCalicoReconcile(t)
		require.NotNil(t, got.ClusterConfig)
		require.NotNil(t, got.ClusterConfig.Spec)
		require.NotNil(t, got.ClusterConfig.Spec.Storage)
		assert.False(t, got.ClusterConfig.Spec.Storage.IsJoinable(), "node-local Kine storage stays not joinable")
		assert.Equal(t, v1beta1.CalicoModeBIRD, got.ClusterConfig.Spec.Network.Calico.Mode, "startup Calico mode stays on the node-local config")
	})

	t.Run("status JSON keeps the node-local Kine config", func(t *testing.T) {
		t.Parallel()
		encoded, err := json.Marshal(statusAfterCalicoReconcile(t))
		require.NoError(t, err)
		var decoded K0sStatus
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		require.NotNil(t, decoded.ClusterConfig)
		require.NotNil(t, decoded.ClusterConfig.Spec)
		require.NotNil(t, decoded.ClusterConfig.Spec.Storage)
		assert.False(t, decoded.ClusterConfig.Spec.Storage.IsJoinable(), "encoded status keeps the node-local Kine storage")
	})

	t.Run("workloads probe failure still reports CNI", func(t *testing.T) {
		t.Parallel()
		st := &Status{
			StatusInformation: K0sStatus{Workloads: true},
			GetWorkerClientFactory: func() kubeutil.ClientFactoryInterface {
				return &failingClientFactory{err: errors.New("kubelet kubeconfig missing")}
			},
			ControllerClientFactory: testutil.NewFakeClientFactory(kubeRouterDS(1, 1)),
		}
		reconcile(t, st, kubeRouterConfig())
		got := fetchStatus(t, st)
		assert.False(t, got.WorkerToAPIConnectionStatus.Success)
		assert.Contains(t, got.WorkerToAPIConnectionStatus.Message, "kubelet kubeconfig missing")
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionTrue, got.Conditions[0].Status)
	})

	t.Run("worker probe and CNI check run together", func(t *testing.T) {
		t.Parallel()

		release := make(chan struct{})
		var releaseOnce sync.Once
		unblock := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(unblock)

		var started sync.WaitGroup
		started.Add(2)

		factory := testutil.NewFakeClientFactory(kubeRouterDS(1, 1))
		factory.Client.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			started.Done()
			<-release
			return false, nil, nil
		})

		st := &Status{
			StatusInformation: K0sStatus{Workloads: true},
			GetWorkerClientFactory: func() kubeutil.ClientFactoryInterface {
				return &blockingClientFactory{started: &started, release: release}
			},
			ControllerClientFactory: factory,
		}
		reconcile(t, st, kubeRouterConfig())
		require.NoError(t, st.Init(t.Context()))

		done := make(chan statusResult, 1)
		go func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/status", nil)
			st.httpserver.Handler.ServeHTTP(rec, req)
			var got K0sStatus
			err := json.Unmarshal(rec.Body.Bytes(), &got)
			done <- statusResult{status: got, err: err}
		}()

		overlapped := make(chan struct{})
		go func() {
			started.Wait()
			close(overlapped)
		}()
		select {
		case <-overlapped:
		case <-time.After(3 * time.Second):
			t.Fatal("worker probe and CNI check did not run together")
		}
		unblock()

		select {
		case res := <-done:
			require.NoError(t, res.err)
			assert.Contains(t, res.status.WorkerToAPIConnectionStatus.Message, "stopped")
			require.Len(t, res.status.Conditions, 1)
			assert.Equal(t, corev1.ConditionTrue, res.status.Conditions[0].Status)
		case <-time.After(3 * time.Second):
			t.Fatal("status request did not finish")
		}
	})
}

type statusResult struct {
	status K0sStatus
	err    error
}

func kubeRouterCondition(t *testing.T, objects ...runtime.Object) Condition {
	t.Helper()
	return reconciledCNIStatus(t, &Status{
		ControllerClientFactory: testutil.NewFakeClientFactory(objects...),
	}, kubeRouterConfig())
}

func calicoCondition(t *testing.T, mode v1beta1.CalicoMode, hasWindows *bool, objects ...runtime.Object) Condition {
	t.Helper()
	return reconciledCNIStatus(t, &Status{
		ControllerClientFactory: testutil.NewFakeClientFactory(objects...),
		HasWindowsNodes:         windowsCounter(hasWindows),
	}, clusterConfigWithCalico(mode))
}

func reconcile(t *testing.T, st *Status, cfg *v1beta1.ClusterConfig) {
	t.Helper()
	require.NoError(t, st.ClusterConfigReconciler().(manager.Reconciler).Reconcile(t.Context(), cfg))
}

func fetchStatus(t *testing.T, st *Status) K0sStatus {
	t.Helper()
	require.NoError(t, st.Init(t.Context()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	st.httpserver.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var got K0sStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func reconciledCNIStatus(t *testing.T, st *Status, cfg *v1beta1.ClusterConfig) Condition {
	t.Helper()
	reconcile(t, st, cfg)
	got := fetchStatus(t, st)
	require.Len(t, got.Conditions, 1)
	return got.Conditions[0]
}

func kubeRouterConfig() *v1beta1.ClusterConfig {
	return clusterConfigWithProvider(constant.CNIProviderKubeRouter)
}

func kubeRouterDS(desired, available int32) *appsv1.DaemonSet {
	return daemonSet(kubeRouterDaemonSet, desired, available)
}

func calicoNodeDS(desired, available int32) *appsv1.DaemonSet {
	return daemonSet(calicoNodeDaemonSet, desired, available)
}

func calicoWindowsDS(desired, available int32) *appsv1.DaemonSet {
	return daemonSet(calicoWindowsDaemonSet, desired, available)
}

func daemonSet(name string, desired, available int32) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		Name:       name,
		Namespace:  metav1.NamespaceSystem,
		Generation: 1,
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			NumberAvailable:        available,
			UpdatedNumberScheduled: desired,
			ObservedGeneration:     1,
		},
	}
}

func calicoControllers(available corev1.ConditionStatus) *appsv1.Deployment {
	return &appsv1.Deployment{
		Name:       calicoControllersDeployment,
		Namespace:  metav1.NamespaceSystem,
		Generation: 1,
		Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			Replicas:           1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			ReadyReplicas:      1,
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: available,
			}},
		},
	}
}

func clusterConfigWithProvider(provider string) *v1beta1.ClusterConfig {
	return &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			Network: &v1beta1.Network{Provider: provider},
		},
	}
}

func clusterConfigWithCalico(mode v1beta1.CalicoMode) *v1beta1.ClusterConfig {
	return &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			Network: calicoNet(mode),
		},
	}
}

func calicoNet(mode v1beta1.CalicoMode) *v1beta1.Network {
	n := &v1beta1.Network{Provider: constant.CNIProviderCalico}
	if mode != "" {
		n.Calico = &v1beta1.Calico{Mode: mode}
	}
	return n
}

type failingClientFactory struct {
	kubeutil.ClientFactoryInterface
	err error
}

func (f *failingClientFactory) GetClient() (kubernetes.Interface, error) {
	return nil, f.err
}

type blockingClientFactory struct {
	kubeutil.ClientFactoryInterface
	started *sync.WaitGroup
	release <-chan struct{}
}

func (f *blockingClientFactory) GetClient() (kubernetes.Interface, error) {
	f.started.Done()
	<-f.release
	return nil, errors.New("stopped")
}

func windowsCounter(present *bool) func() (*bool, <-chan struct{}) {
	return func() (*bool, <-chan struct{}) { return present, nil }
}

func statusAfterCalicoReconcile(t *testing.T) K0sStatus {
	t.Helper()
	startup := clusterConfigWithCalico(v1beta1.CalicoModeBIRD)
	startup.Spec.Storage = &v1beta1.StorageSpec{
		Type: v1beta1.KineStorageType,
		Kine: &v1beta1.KineConfig{DataSource: "sqlite:///var/lib/k0s/db/state.db"},
	}
	st := &Status{StatusInformation: K0sStatus{ClusterConfig: startup}}
	reconcile(t, st, clusterConfigWithCalico(v1beta1.CalicoModeVXLAN))
	return fetchStatus(t, st)
}
