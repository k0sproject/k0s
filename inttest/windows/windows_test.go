// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"context"
	"os"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const testNamespace = "windows-test"

// This test works on an existing cluster where there's 1 or more Windows nodes
// We test few things:
// - Node readiness
// - Pod scheduling; check that all expected components are running (mainly Calico & kube-proxy)
// TODO
// - Pod-to-pod networking works across Windows and Linux nodes
// pod-to-pod is NOT working in CI since Calico borks WSL <--> windows networking
func TestWindows(t *testing.T) {
	require := require.New(t)

	kubeconfig := os.Getenv("KUBECONFIG")
	require.NotEmpty(kubeconfig, "KUBECONFIG must be set for this test to work")
	t.Logf("Using kubeconfig: %s", kubeconfig)
	var err error
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(err, "Failed to build kubeconfig")
	kc, err := kubernetes.NewForConfig(restConfig)
	require.NoError(err, "Failed to create Kubernetes client")

	ver, err := kc.Discovery().ServerVersion()
	require.NoError(err, "Failed to get server version")
	t.Logf("Connected to Kubernetes API server version: %s", ver.GitVersion)

	t.Log("Creating test namespaces")
	_, err = kc.CoreV1().Namespaces().Create(t.Context(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}, metav1.CreateOptions{})

	require.NoError(err, "Failed to create test namespace")
	t.Cleanup(func() {
		t.Logf("Deleting test namespace: %s", testNamespace)
		err := kc.CoreV1().Namespaces().Delete(t.Context(), testNamespace, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Failed to delete test namespace: %v", err)
		} else {
			t.Logf("Test namespace deleted")
		}
	})

	t.Run("Verify nodes come to Ready state", func(t *testing.T) {
		t.Log("Waiting for all nodes to be ready")
		require.NoError(common.Poll(t.Context(), func(ctx context.Context) (bool, error) {
			nodes, err := kc.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			if len(nodes.Items) < 2 {
				t.Logf("Waiting for at least 2 nodes, got %d", len(nodes.Items))
				return false, nil
			}

			for _, node := range nodes.Items {
				// Find the ready condition
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady {
						t.Logf("Node %s condition %s is %s", node.Name, condition.Type, condition.Status)
						if condition.Status != corev1.ConditionTrue {
							return false, nil
						}
					}
				}
			}
			return true, nil
		}))
		t.Log("All nodes are ready")
	})

	t.Run("Verify system services come up", func(t *testing.T) {
		t.Log("Waiting for kube-proxy DaemonSet to be ready")
		require.NoError(common.WaitForDaemonSet(t.Context(), kc, "kube-proxy", metav1.NamespaceSystem))
		t.Log("Waiting for kube-proxy-windows DaemonSet to be ready")
		require.NoError(common.WaitForDaemonSet(t.Context(), kc, "kube-proxy-windows", metav1.NamespaceSystem))
		t.Log("Waiting for calico-node DaemonSet to be ready")
		require.NoError(common.WaitForDaemonSet(t.Context(), kc, "calico-node", metav1.NamespaceSystem))
		t.Log("Waiting for calico-node-windows DaemonSet to be ready")
		require.NoError(common.WaitForDaemonSet(t.Context(), kc, "calico-node-windows", metav1.NamespaceSystem))
		t.Log("All system DaemonSets are ready")
	})

	t.Run("Verify pods can be scheduled on Windows node", func(t *testing.T) {
		t.Log("Creating test pods and services on both Windows and Linux nodes")
		require.NoError(runWindowsDeployment(t.Context(), kc))
		t.Log("Waiting for Windows test pod to be ready")
		require.NoError(common.WaitForPod(t.Context(), kc, "iis", testNamespace))
	})

	t.Run("Verify pods can be scheduled on Linux node", func(t *testing.T) {
		require.NoError(runLinuxDeployment(t.Context(), kc))
		t.Log("Waiting for Linux test pod to be ready")
		require.NoError(common.WaitForPod(t.Context(), kc, "nginx-linux", testNamespace))

	})

}

func runLinuxDeployment(ctx context.Context, kc *kubernetes.Clientset) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "nginx-linux",
			Labels: map[string]string{"app": "nginx-linux"},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{corev1.LabelOSStable: string(corev1.Linux)},
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}
	_, err := kc.CoreV1().Pods(testNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-linux-svc",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "nginx-linux"},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}
	_, err = kc.CoreV1().Services(testNamespace).Create(ctx, svc, metav1.CreateOptions{})
	return err
}

func runWindowsDeployment(ctx context.Context, kc *kubernetes.Clientset) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "iis",
			Labels: map[string]string{"app": "iis-windows"},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{corev1.LabelOSStable: string(corev1.Windows)},
			Containers: []corev1.Container{
				{
					Name:  "iis",
					Image: "mcr.microsoft.com/windows/servercore/iis",
				},
			},
		},
	}
	_, err := kc.CoreV1().Pods(testNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "iis-windows-svc",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "iis-windows"},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}
	_, err = kc.CoreV1().Services(testNamespace).Create(ctx, svc, metav1.CreateOptions{})
	return err
}
