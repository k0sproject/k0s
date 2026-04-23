// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/bits"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/require"
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
		t.Log("Waiting for konnectivity-agent DaemonSet to be ready")
		require.NoError(common.WaitForDaemonSet(t.Context(), kc, "konnectivity-agent", metav1.NamespaceSystem))
		t.Log("All system DaemonSets are ready")
	})

	t.Run("Verify konnectivity is operational", func(t *testing.T) {
		require.NoError(verifyKonnectivityPods(t.Context(), restConfig, kc, t))
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

func verifyKonnectivityPods(ctx context.Context, config *rest.Config, kc kubernetes.Interface, t *testing.T) error {
	podDialer, err := testutil.NewPodDialer(config)
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: podDialer.DialContext,
		},
	}
	t.Cleanup(client.CloseIdleConnections)

	return wait.PollUntilContextCancel(ctx, 3*time.Second, true, func(ctx context.Context) (bool, error) {
		pods, err := kc.CoreV1().Pods(metav1.NamespaceSystem).List(ctx, metav1.ListOptions{
			LabelSelector: fields.OneTermEqualSelector("k8s-app", "konnectivity-agent").String(),
		})
		if err != nil {
			t.Logf("Failed to get konnectivity pods: %v", err)
			return false, nil
		}

		if len := len(pods.Items); len > 2 {
			return false, fmt.Errorf("unexpected number of konnectivity pods: %d", len)
		}

		var goodPods uint
		for _, pod := range pods.Items {
			openServerConnections, err := fetchOpenServerConnections(ctx, &client, &pod)
			if err != nil {
				t.Logf("Failed to fetch konnectivity metrics from pod %s on node %s: %v", pod.Name, pod.Spec.NodeName, err)
				return false, nil
			}

			switch openServerConnections {
			case 0:
				t.Logf("Pod %s on node %s has no open konnectivity server connections", pod.Name, pod.Spec.NodeName)
			case 1:
				t.Logf("Pod %s on node %s has one open konnectivity server connection", pod.Name, pod.Spec.NodeName)
				goodPods++
			default:
				return false, fmt.Errorf("unexpected number of open server connections for pod %s on node %s: %d", pod.Name, pod.Spec.NodeName, openServerConnections)
			}
		}

		t.Logf("Pods with one open konnectivity server connection: %d/%d", goodPods, len(pods.Items))
		return goodPods == 2, nil
	})
}

func fetchOpenServerConnections(ctx context.Context, client *http.Client, pod *corev1.Pod) (_ uint, err error) {
	var port uint16
	for k, v := range pod.Annotations {
		if k == "prometheus.io/port" {
			parsed, err := strconv.ParseUint(v, 10, 16)
			if err != nil {
				return 0, fmt.Errorf("invalid port: %w", err)
			}
			if parsed == 0 {
				return 0, errors.New("zero port")
			}
			port = uint16(parsed)
		}
	}

	if port == 0 {
		return 0, errors.New("no port")
	}

	url := fmt.Sprintf("http://%s.%s:%d/metrics", pod.Name, pod.Namespace, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send HTTP request: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close HTTP response body: %w", closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("non-OK HTTP response status: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		key, val, found := bytes.Cut(scanner.Bytes(), []byte{' '})
		if !found || !bytes.Equal(key, []byte("konnectivity_network_proxy_agent_open_server_connections")) {
			continue
		}

		val, _, _ = bytes.Cut(val, []byte{' '})
		count, err := strconv.ParseUint(string(val), 10, bits.UintSize)
		if err != nil {
			return 0, fmt.Errorf("invalid metric: %s %s: %w", key, val, err)
		}
		return uint(count), nil
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	return 0, errors.New("metric konnectivity_network_proxy_agent_open_server_connections not found")
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
