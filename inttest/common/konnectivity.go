// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/bits"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
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

	type monitor struct {
		node, pod string
		cancel    context.CancelCauseFunc
		mu        sync.Mutex
	}

	var (
		monitors []*monitor
		goodPods atomic.Int32
	)

	allGood := errors.New("all good")
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return watch.Pods(kc.CoreV1().Pods(metav1.NamespaceSystem)).
			WithLabels(labels.Set{"k8s-app": "konnectivity-agent"}).
			WithErrorCallback(RetryWatchErrors(t.Logf)).
			IncludingDeletions().
			Until(ctx, func(pod *corev1.Pod) (bool, error) {
				var badMsg string
				if pod.DeletionTimestamp != nil {
					badMsg = "pod deleted"
				} else if idx := slices.IndexFunc(pod.Status.Conditions, func(cond corev1.PodCondition) bool {
					return cond.Type == corev1.PodReady
				}); idx < 0 || pod.Status.Conditions[idx].Status != corev1.ConditionTrue {
					badMsg = "pod not ready"
				} else if pod.Spec.NodeName == "" {
					badMsg = "pod is not assigned to a node"
				}

				if badMsg != "" {
					for _, monitor := range monitors {
						if monitor.pod == pod.Name {
							if monitor.cancel != nil {
								monitor.cancel(errors.New(badMsg))
								monitor.cancel = nil
							}
							break
						}
					}
					return false, nil
				}

				var monitorForNode *monitor
				for _, monitor := range monitors {
					if monitor.node == pod.Spec.NodeName {
						monitorForNode = monitor
						break
					}
				}
				if monitorForNode == nil {
					monitorForNode = &monitor{node: pod.Spec.NodeName, pod: pod.Name}
					monitors = append(monitors, monitorForNode)
				} else if monitorForNode.pod != pod.Name {
					if monitorForNode.cancel != nil {
						monitorForNode.cancel(errors.New("pod has been replaced"))
					}
					monitorForNode.pod = pod.Name
				} else if monitorForNode.cancel != nil {
					return false, nil
				}

				ctx, cancelPod := context.WithCancelCause(ctx)
				monitorForNode.cancel = func(cause error) {
					t.Logf("Canceling monitoring konnectivity metrics from %s on %s: %v", monitorForNode.pod, monitorForNode.node, cause)
					cancelPod(cause)
				}

				eg.Go(func() (err error) {
					monitorForNode.mu.Lock()
					defer monitorForNode.mu.Unlock()

					t.Logf("Monitoring konnectivity metrics from %s on %s", pod.Name, pod.Spec.NodeName)

					var (
						openServerConnections uint
						good                  bool
						lastErrMsg            string
					)
					defer func() {
						if good {
							numGood := goodPods.Add(-1)
							if err != nil && !errors.Is(err, allGood) {
								t.Logf(
									"Fully connected konnectivity agents: %d/%d (%s on %s: %v)",
									numGood, numWorkers, pod.Name, pod.Spec.NodeName, err,
								)
							}
						}
					}()

					for {
						select {
						case <-time.After(1 * time.Second):
						case <-ctx.Done():
							return nil
						}

						if conns, err := func() (uint, error) {
							ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
							defer cancel()
							return fetchOpenKonnectivityServerConnections(ctx, &client, pod)
						}(); err != nil {
							// More concise message for an expected failure case.
							if errors.Is(err, testutil.ErrNoKonnectivityAgent) {
								err = testutil.ErrNoKonnectivityAgent
							}
							if errMsg := err.Error(); errMsg != lastErrMsg {
								t.Logf("Failed to fetch konnectivity metrics from %s on node %s: %s", pod.Name, pod.Spec.NodeName, errMsg)
								lastErrMsg = errMsg
							}
							if good {
								good = false
								goodPods.Add(-1)
							}
							continue
						} else {
							if conns == openServerConnections && lastErrMsg == "" {
								continue
							}
							openServerConnections, lastErrMsg = conns, ""
						}

						t.Logf("Open konnectivity server connections for %s on %s: %d/%d", pod.Name, pod.Spec.NodeName, openServerConnections, numControllers)
						if openServerConnections == numControllers {
							if !good {
								good = true
								numGood := uint(goodPods.Add(1))
								t.Logf("Fully connected konnectivity agents: %d/%d", numGood, numWorkers)
								if numGood == numWorkers {
									return allGood
								}
							}
						} else if good {
							good = false
							goodPods.Add(-1)
						}
					}
				})

				return false, nil
			})
	})

	if err := eg.Wait(); !errors.Is(err, allGood) {
		return cmp.Or(err, ctx.Err())
	}

	t.Log("Konnectivity mesh is complete")
	return nil
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
