// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEvaluateCNI_CustomProviderSkipped(t *testing.T) {
	t.Parallel()
	_, ok := evaluateCNI(t.Context(), fake.NewSimpleClientset(), &v1beta1.Network{Provider: "custom"})
	assert.False(t, ok)
}

func TestEvaluateCNI_KubeRouter(t *testing.T) {
	t.Parallel()

	t.Run("healthy", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(kubeRouterDS(1, 1))
		got, ok := evaluateCNI(t.Context(), client, kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, Condition{
			Type:    conditionCNIReady,
			Status:  corev1.ConditionTrue,
			Reason:  reasonAllReady,
			Message: "kube-router DaemonSet reports 1/1 available, 1 updated",
		}, got)
	})

	t.Run("not_ready", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(kubeRouterDS(1, 0))
		got, ok := evaluateCNI(t.Context(), client, kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "kube-router DaemonSet reports 0/1 available, 1 updated", got.Message)
	})

	t.Run("zero_desired_is_not_ready", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(kubeRouterDS(0, 0))
		got, ok := evaluateCNI(t.Context(), client, kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		got, ok := evaluateCNI(t.Context(), fake.NewSimpleClientset(), kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotFound, got.Reason)
		assert.Equal(t, "kube-router DaemonSet not found", got.Message)
	})

	t.Run("api_error", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset()
		client.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver unavailable")
		})
		got, ok := evaluateCNI(t.Context(), client, kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, "apiserver unavailable", got.Message)
	})

	t.Run("stale_generation", func(t *testing.T) {
		t.Parallel()
		ds := kubeRouterDS(1, 1)
		ds.Generation = 2
		ds.Status.ObservedGeneration = 1
		got, ok := evaluateCNI(t.Context(), fake.NewSimpleClientset(ds), kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "kube-router DaemonSet has not observed generation 2 (observed 1)", got.Message)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset()
		client.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, context.DeadlineExceeded
		})
		got, ok := evaluateCNI(t.Context(), client, kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonTimeout, got.Reason)
		assert.Equal(t, "CNI status check timed out", got.Message)
	})

	t.Run("canceled_is_error_not_timeout", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset()
		client.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, context.Canceled
		})
		got, ok := evaluateCNI(t.Context(), client, kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, context.Canceled.Error(), got.Message)
	})

	t.Run("old_pods_during_rollout", func(t *testing.T) {
		t.Parallel()
		ds := kubeRouterDS(2, 2)
		ds.Status.UpdatedNumberScheduled = 1
		got, ok := evaluateCNI(t.Context(), fake.NewSimpleClientset(ds), kubeRouterNet())
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "kube-router DaemonSet reports 2/2 available, 1 updated", got.Message)
	})
}

func TestEvaluateCNI_Calico(t *testing.T) {
	t.Parallel()

	t.Run("healthy", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(2, 2), calicoControllers(corev1.ConditionTrue))
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.Equal(t, reasonAllReady, got.Reason)
		assert.Equal(t, "calico-node DaemonSet reports 2/2 available, 2 updated; calico-kube-controllers Deployment is available", got.Message)
	})

	t.Run("node_not_ready", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(2, 1), calicoControllers(corev1.ConditionTrue))
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-node DaemonSet reports 1/2 available, 2 updated", got.Message)
	})

	t.Run("controllers_not_ready", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1), calicoControllers(corev1.ConditionFalse))
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonComponentsNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment is not available", got.Message)
	})

	t.Run("controllers_not_found", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1))
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotFound, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment not found", got.Message)
	})

	t.Run("controllers_api_error", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1))
		client.PrependReactor("get", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver unavailable")
		})
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, "apiserver unavailable", got.Message)
	})

	t.Run("prefers_false_over_unknown", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(1, 0))
		client.PrependReactor("get", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver unavailable")
		})
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-node DaemonSet reports 0/1 available, 1 updated", got.Message)
	})

	t.Run("windows_not_found_is_ignored", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1), calicoControllers(corev1.ConditionTrue))
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.NotContains(t, got.Message, calicoWindowsDaemonSet)
	})

	t.Run("windows_not_ready", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(
			calicoNodeDS(1, 1),
			calicoWindowsDS(1, 0),
			calicoControllers(corev1.ConditionTrue),
			windowsNode(),
		)
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotReady, got.Reason)
		assert.Equal(t, "calico-node-windows DaemonSet reports 0/1 available, 1 updated", got.Message)
	})

	t.Run("windows_ready", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(
			calicoNodeDS(1, 1),
			calicoWindowsDS(1, 1),
			calicoControllers(corev1.ConditionTrue),
			windowsNode(),
		)
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.Equal(t, "calico-node DaemonSet reports 1/1 available, 1 updated; calico-node-windows DaemonSet reports 1/1 available, 1 updated; calico-kube-controllers Deployment is available", got.Message)
	})

	t.Run("controllers_unknown", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1), calicoControllers(corev1.ConditionUnknown))
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionUnknown, got.Status)
		assert.Equal(t, reasonError, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment availability is unknown", got.Message)
	})

	t.Run("windows_required_when_windows_nodes_exist", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(
			calicoNodeDS(1, 1),
			calicoControllers(corev1.ConditionTrue),
			windowsNode(),
		)
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonNotFound, got.Reason)
		assert.Equal(t, "calico-node-windows DaemonSet not found", got.Message)
	})

	t.Run("bird_mode_does_not_require_windows_daemonset", func(t *testing.T) {
		t.Parallel()
		client := fake.NewSimpleClientset(
			calicoNodeDS(1, 1),
			calicoControllers(corev1.ConditionTrue),
			windowsNode(),
		)
		got, ok := evaluateCNI(t.Context(), client, calicoNet(v1beta1.CalicoModeBIRD))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionTrue, got.Status)
		assert.NotContains(t, got.Message, calicoWindowsDaemonSet)
	})

	t.Run("controllers_stale_generation", func(t *testing.T) {
		t.Parallel()
		deploy := calicoControllers(corev1.ConditionTrue)
		deploy.Generation = 3
		deploy.Status.ObservedGeneration = 2
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1), deploy)
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonComponentsNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment has not observed generation 3 (observed 2)", got.Message)
	})

	t.Run("controllers_old_replicas_during_rollout", func(t *testing.T) {
		t.Parallel()
		deploy := calicoControllers(corev1.ConditionTrue)
		deploy.Status.UpdatedReplicas = 0
		deploy.Status.AvailableReplicas = 1
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1), deploy)
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonComponentsNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment reports 0/1 updated, 1 available, 1 total", got.Message)
	})

	t.Run("controllers_old_pods_remain", func(t *testing.T) {
		t.Parallel()
		deploy := calicoControllers(corev1.ConditionTrue)
		deploy.Status.Replicas = 2
		deploy.Status.UpdatedReplicas = 1
		deploy.Status.AvailableReplicas = 1
		client := fake.NewSimpleClientset(calicoNodeDS(1, 1), deploy)
		got, ok := evaluateCNI(t.Context(), client, calicoNet(""))
		require.True(t, ok)
		assert.Equal(t, corev1.ConditionFalse, got.Status)
		assert.Equal(t, reasonComponentsNotReady, got.Reason)
		assert.Equal(t, "calico-kube-controllers Deployment reports 1/1 updated, 1 available, 2 total", got.Message)
	})
}

func TestAppendCNICondition(t *testing.T) {
	t.Parallel()

	t.Run("skips_without_admin_factory", func(t *testing.T) {
		t.Parallel()
		status := K0sStatus{ClusterConfig: clusterConfigWithProvider(constant.CNIProviderKubeRouter)}
		sh := &statusHandler{Status: &Status{}}
		sh.appendCNICondition(t.Context(), &status)
		assert.Empty(t, status.Conditions)
	})

	t.Run("skips_custom_cni", func(t *testing.T) {
		t.Parallel()
		status := K0sStatus{ClusterConfig: clusterConfigWithProvider("custom")}
		sh := &statusHandler{Status: &Status{
			AdminClientFactory: &stubFactory{client: fake.NewSimpleClientset()},
		}}
		sh.appendCNICondition(t.Context(), &status)
		assert.Empty(t, status.Conditions)
	})

	t.Run("skips_custom_cni_when_admin_client_fails", func(t *testing.T) {
		t.Parallel()
		status := K0sStatus{ClusterConfig: clusterConfigWithProvider("custom")}
		sh := &statusHandler{Status: &Status{
			AdminClientFactory: &stubFactory{err: errors.New("admin kubeconfig missing")},
		}}
		sh.appendCNICondition(t.Context(), &status)
		assert.Empty(t, status.Conditions)
	})

	t.Run("skips_worker_without_cluster_config", func(t *testing.T) {
		t.Parallel()
		status := K0sStatus{}
		sh := &statusHandler{Status: &Status{
			AdminClientFactory: &stubFactory{client: fake.NewSimpleClientset()},
		}}
		sh.appendCNICondition(t.Context(), &status)
		assert.Empty(t, status.Conditions)
	})

	t.Run("admin_client_error", func(t *testing.T) {
		t.Parallel()
		status := K0sStatus{ClusterConfig: clusterConfigWithProvider(constant.CNIProviderKubeRouter)}
		sh := &statusHandler{Status: &Status{
			AdminClientFactory: &stubFactory{err: errors.New("admin kubeconfig missing")},
		}}
		sh.appendCNICondition(t.Context(), &status)
		require.Len(t, status.Conditions, 1)
		assert.Equal(t, corev1.ConditionUnknown, status.Conditions[0].Status)
		assert.Equal(t, reasonError, status.Conditions[0].Reason)
		assert.Equal(t, "admin kubeconfig missing", status.Conditions[0].Message)
	})

	t.Run("controller_without_workloads_still_checks_cni", func(t *testing.T) {
		t.Parallel()
		sh := &statusHandler{Status: &Status{
			StatusInformation: K0sStatus{
				Workloads:     false,
				ClusterConfig: clusterConfigWithProvider(constant.CNIProviderKubeRouter),
			},
			AdminClientFactory: &stubFactory{client: fake.NewSimpleClientset(kubeRouterDS(1, 1))},
		}}
		got := sh.getCurrentStatus(t.Context())
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionTrue, got.Conditions[0].Status)
		assert.False(t, got.WorkerToAPIConnectionStatus.Success)
	})

	t.Run("reconciled_calico_mode_requires_windows_daemonset", func(t *testing.T) {
		t.Parallel()
		startup := clusterConfigWithCalico(v1beta1.CalicoModeBIRD)
		startup.Spec.Storage = &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
			Kine: &v1beta1.KineConfig{DataSource: "sqlite:///var/lib/k0s/db/state.db"},
		}
		st := &Status{
			StatusInformation: K0sStatus{ClusterConfig: startup},
			AdminClientFactory: &stubFactory{client: fake.NewSimpleClientset(
				calicoNodeDS(1, 1),
				calicoControllers(corev1.ConditionTrue),
				windowsNode(),
			)},
		}
		sh := &statusHandler{Status: st}

		got := sh.getCurrentStatus(t.Context())
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionTrue, got.Conditions[0].Status)

		clusterWide := clusterConfigWithCalico(v1beta1.CalicoModeVXLAN)
		require.NoError(t, (&ClusterConfigUpdater{Status: st}).Reconcile(t.Context(), clusterWide))
		got = sh.getCurrentStatus(t.Context())
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionFalse, got.Conditions[0].Status)
		assert.Equal(t, reasonNotFound, got.Conditions[0].Reason)
		assert.Equal(t, "calico-node-windows DaemonSet not found", got.Conditions[0].Message)
		require.NotNil(t, got.ClusterConfig)
		require.NotNil(t, got.ClusterConfig.Spec)
		require.NotNil(t, got.ClusterConfig.Spec.Storage)
		assert.False(t, got.ClusterConfig.Spec.Storage.IsJoinable())
		assert.Equal(t, v1beta1.CalicoModeBIRD, got.ClusterConfig.Spec.Network.Calico.Mode)

		encoded, err := json.Marshal(got)
		require.NoError(t, err)
		var decoded K0sStatus
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		require.NotNil(t, decoded.ClusterConfig)
		require.NotNil(t, decoded.ClusterConfig.Spec)
		require.NotNil(t, decoded.ClusterConfig.Spec.Storage)
		assert.False(t, decoded.ClusterConfig.Spec.Storage.IsJoinable())
	})

	t.Run("workloads_probe_failure_still_reports_cni", func(t *testing.T) {
		t.Parallel()
		sh := &statusHandler{Status: &Status{
			StatusInformation: K0sStatus{
				Workloads:     true,
				ClusterConfig: clusterConfigWithProvider(constant.CNIProviderKubeRouter),
			},
			GetWorkerClientFactory: func() kubeutil.ClientFactoryInterface {
				return &stubFactory{err: errors.New("kubelet kubeconfig missing")}
			},
			AdminClientFactory: &stubFactory{client: fake.NewSimpleClientset(kubeRouterDS(1, 1))},
		}}
		got := sh.getCurrentStatus(t.Context())
		assert.False(t, got.WorkerToAPIConnectionStatus.Success)
		assert.Contains(t, got.WorkerToAPIConnectionStatus.Message, "kubelet kubeconfig missing")
		require.Len(t, got.Conditions, 1)
		assert.Equal(t, corev1.ConditionTrue, got.Conditions[0].Status)
	})
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceSystem, Generation: 1},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			NumberAvailable:        available,
			UpdatedNumberScheduled: desired,
			ObservedGeneration:     1,
		},
	}
}

func calicoControllers(available corev1.ConditionStatus) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: calicoControllersDeployment, Namespace: metav1.NamespaceSystem, Generation: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			Replicas:           replicas,
			UpdatedReplicas:    replicas,
			AvailableReplicas:  replicas,
			ReadyReplicas:      replicas,
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: available,
			}},
		},
	}
}

func kubeRouterNet() *v1beta1.Network {
	return &v1beta1.Network{Provider: constant.CNIProviderKubeRouter}
}

func calicoNet(mode v1beta1.CalicoMode) *v1beta1.Network {
	n := &v1beta1.Network{Provider: constant.CNIProviderCalico}
	if mode != "" {
		n.Calico = &v1beta1.Calico{Mode: mode}
	}
	return n
}

func windowsNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "win-1",
			Labels: map[string]string{corev1.LabelOSStable: string(corev1.Windows)},
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

type stubFactory struct {
	kubeutil.ClientFactoryInterface
	client kubernetes.Interface
	err    error
}

func (s *stubFactory) GetClient() (kubernetes.Interface, error) {
	return s.client, s.err
}
