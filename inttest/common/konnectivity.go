// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/bits"
	"net/http"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/k0sproject/k0s/internal/testutil"
)

func VerifyKonnectivityMesh(ctx context.Context, config *rest.Config, kc kubernetes.Interface, t *testing.T, numControllers, numWorkers uint) error {
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

		if len := uint(len(pods.Items)); len > numWorkers {
			return false, fmt.Errorf("unexpected number of konnectivity pods: %d", len)
		}

		var goodPods uint
		for _, pod := range pods.Items {
			openServerConnections, err := fetchOpenKonnectivityServerConnections(ctx, &client, &pod)
			if err != nil {
				t.Logf("Failed to fetch konnectivity metrics from pod %s on node %s: %v", pod.Name, pod.Spec.NodeName, err)
				return false, nil
			}

			t.Logf("Open konnectivity server connections for %s on %s: %d", pod.Name, pod.Spec.NodeName, openServerConnections)
			if openServerConnections > numControllers {
				return false, fmt.Errorf("too many open server connections for pod %s on node %s: %d exceeds the number of controllers (%d)", pod.Name, pod.Spec.NodeName, openServerConnections, numControllers)
			}
			if openServerConnections == numControllers {
				goodPods++
			}
		}

		t.Logf("Pods with the desired amount of konnectivity server connections: %d/%d", goodPods, len(pods.Items))
		return goodPods == numWorkers, nil
	})
}

func fetchOpenKonnectivityServerConnections(ctx context.Context, client *http.Client, pod *corev1.Pod) (_ uint, err error) {
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
