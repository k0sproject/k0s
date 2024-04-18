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

package nllb

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	testifysuite "github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"
)

const kubeSystem = "kube-system"

type suite struct {
	common.BootlooseSuite
}

func (s *suite) TestNodeLocalLoadBalancing() {
	const controllerArgs = "--kube-controller-manager-extra-args='--node-monitor-period=3s --node-monitor-grace-period=9s'"

	ctx := s.Context()

	{
		config, err := yaml.Marshal(&v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				Network: func() *v1beta1.Network {
					network := v1beta1.DefaultNetwork()
					network.NodeLocalLoadBalancing.Enabled = true
					return network
				}(),

				WorkerProfiles: v1beta1.WorkerProfiles{
					v1beta1.WorkerProfile{
						Name: "default",
						Config: func() *runtime.RawExtension {
							kubeletConfig := kubeletv1beta1.KubeletConfiguration{
								NodeStatusUpdateFrequency: metav1.Duration{Duration: 3 * time.Second},
							}
							bytes, err := json.Marshal(kubeletConfig)
							s.Require().NoError(err)
							return &runtime.RawExtension{Raw: bytes}
						}(),
					},
				},
			},
		})
		s.Require().NoError(err)

		for i := 0; i < s.ControllerCount; i++ {
			s.WriteFileContent(s.ControllerNode(i), "/tmp/k0s.yaml", config)
		}
	}

	s.Run("controller_and_workers_get_up", func() {
		s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", controllerArgs))

		s.T().Logf("Starting workers and waiting for cluster to become ready")

		token, err := s.GetJoinToken("worker")
		s.Require().NoError(err)
		s.Require().NoError(s.RunWorkersWithToken(token))

		clients, err := s.KubeClient(s.ControllerNode(0))
		s.Require().NoError(err)

		eg, _ := errgroup.WithContext(ctx)
		for i := 0; i < s.WorkerCount; i++ {
			nodeName := s.WorkerNode(i)
			eg.Go(func() error {
				if err := s.WaitForNodeReady(nodeName, clients); err != nil {
					return fmt.Errorf("Node %s is not ready: %w", nodeName, err)
				}
				return nil
			})
		}
		s.Require().NoError(eg.Wait())

		s.Require().NoError(s.checkClusterReadiness(ctx, clients, 1))
	})

	s.Run("join_new_controllers", func() {
		token, err := s.GetJoinToken("controller")
		s.Require().NoError(err)

		eg, _ := errgroup.WithContext(ctx)
		eg.Go(func() error { return s.InitController(1, "--config=/tmp/k0s.yaml", controllerArgs, token) })
		eg.Go(func() error { return s.InitController(2, "--config=/tmp/k0s.yaml", controllerArgs, token) })

		s.Require().NoError(eg.Wait())

		clients, err := s.KubeClient(s.ControllerNode(1))
		s.Require().NoError(err)

		s.T().Logf("Checking if HA cluster is ready")
		s.Require().NoError(s.checkClusterReadiness(ctx, clients, s.ControllerCount))
	})

	workerNameToRestart := s.WorkerNode(0)
	for i := 0; i < s.ControllerCount; i++ {
		controllerName := s.ControllerNode(i)
		s.Run(fmt.Sprintf("stop_%s_before_%s", workerNameToRestart, controllerName), func() {
			err := s.StopWorker(workerNameToRestart)
			s.Require().NoError(err)

			clients, err := s.KubeClient(controllerName)
			s.Require().NoError(err)

			s.Require().NoError(
				common.WaitForNodeReadyStatus(ctx, clients, workerNameToRestart, corev1.ConditionUnknown),
				"Didn't observe node %s to be non-ready", workerNameToRestart,
			)
		})

		s.Run("stop_"+controllerName, func() {
			clients, err := s.KubeClient(controllerName)
			s.Require().NoError(err)

			_, err = clients.ServerVersion()
			s.Require().NoError(err)

			err = s.StopController(controllerName)
			s.Require().NoError(err)

			_, err = clients.ServerVersion()
			s.Require().Error(err)
		})

		s.Run(fmt.Sprintf("restart_%s_without_%s", workerNameToRestart, controllerName), func() {
			err := s.StartWorker(workerNameToRestart)
			s.Require().NoError(err)
		})

		clients, err := s.KubeClient(s.ControllerNode((i + 1) % s.ControllerCount))
		s.Require().NoError(err)

		s.Run("cluster_ready_without_"+controllerName, func() {
			s.Require().NoError(s.checkClusterReadiness(ctx, clients, s.ControllerCount, controllerName))
		})

		s.Run("workloads_still_runnable_without_"+controllerName, func() {
			name := "dummy-" + controllerName
			pauseImage := (&v1beta1.ImageSpec{Image: constant.KubePauseContainerImage, Version: constant.KubePauseContainerImageVersion}).URI()
			labels := map[string]string{"dummy": controllerName}
			dummyDaemons := appsv1.DaemonSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1", Kind: "DaemonSet",
				},
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: labels},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "dummy", Image: pauseImage}},
						},
					},
				},
			}

			_, err = clients.AppsV1().DaemonSets("kube-system").Create(ctx, &dummyDaemons, metav1.CreateOptions{})
			s.Require().NoError(err)

			s.NoError(common.WaitForDaemonSet(s.Context(), clients, name, "kube-system"))
			s.T().Logf("Dummy DaemonSet %s is ready", name)
		})

		s.Run("restart_"+controllerName, func() {
			s.Require().NoError(s.StartController(controllerName))
			clients, err := s.KubeClient(controllerName)
			s.Require().NoError(err)
			s.Require().NoError(s.checkClusterReadiness(ctx, clients, s.ControllerCount))
		})
	}

	s.Run("cluster_ready_after_all_controllers_restarted", func() {
		clients, err := s.KubeClient(s.ControllerNode(0))
		s.Require().NoError(err)
		s.Require().NoError(s.checkClusterReadiness(ctx, clients, s.ControllerCount))
	})
}

func (s *suite) checkClusterReadiness(ctx context.Context, clients *kubernetes.Clientset, numControllers int, degradedControllers ...string) error {
	eg, ctx := errgroup.WithContext(ctx)

	for i := 0; i < numControllers; i++ {
		nodeName := s.ControllerNode(i)
		degraded := slices.Contains(degradedControllers, nodeName)

		eg.Go(func() error {
			var holderIdentity string
			watchLeases := watch.FromClient[*coordinationv1.LeaseList, coordinationv1.Lease]

			err := watchLeases(clients.CoordinationV1().Leases("kube-node-lease")).
				WithObjectName("k0s-ctrl-"+nodeName).
				WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
				Until(ctx, func(lease *coordinationv1.Lease) (bool, error) {
					holderIdentity = *lease.Spec.HolderIdentity
					return (degraded && holderIdentity == "") || (!degraded && holderIdentity != ""), nil
				})
			if err != nil {
				return fmt.Errorf("while watching k0s controller lease for %s: %w", nodeName, err)
			}

			s.T().Logf("K0s controller lease for %s: %q", nodeName, holderIdentity)
			return nil
		})
	}

	for i := 0; i < s.WorkerCount; i++ {
		nodeName := s.WorkerNode(i)

		eg.Go(func() error {
			if err := common.WaitForNodeReadyStatus(ctx, clients, nodeName, corev1.ConditionTrue); err != nil {
				return fmt.Errorf("node %s did not become ready: %w", nodeName, err)
			}

			s.T().Logf("Node %s is ready", nodeName)

			nllbPodName := fmt.Sprintf("nllb-%s", nodeName)
			if err := common.WaitForPod(ctx, clients, nllbPodName, kubeSystem); err != nil {
				return fmt.Errorf("Pod %s/%s is not ready: %w", nllbPodName, kubeSystem, err)
			}
			s.T().Logf("Pod %s/%s is ready", kubeSystem, nllbPodName)

			// Test that we get logs, it's a signal that konnectivity tunnels work.
			var logsErr error
			if err := wait.PollImmediateUntilWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				logs, err := clients.CoreV1().Pods(kubeSystem).GetLogs(nllbPodName, &corev1.PodLogOptions{}).Stream(ctx)
				if err != nil {
					if logsErr == nil || err.Error() != logsErr.Error() {
						s.T().Logf("No logs yet from %s/%s: %v", kubeSystem, nllbPodName, err)
					}
					logsErr = err
					return false, nil
				}
				return true, logs.Close()
			}); err != nil {
				return fmt.Errorf("failed to get pod logs from %s/%s: %w", kubeSystem, nllbPodName, logsErr)
			}

			s.T().Logf("Got some pod logs from %s/%s", kubeSystem, nllbPodName)
			return nil
		})
	}

	eg.Go(func() error {
		if err := common.WaitForKubeRouterReady(ctx, clients); err != nil {
			return fmt.Errorf("kube-router did not start: %w", err)
		}
		s.T().Logf("kube-router is ready")
		return nil
	})

	for _, lease := range []string{"kube-scheduler", "kube-controller-manager"} {
		lease := lease
		eg.Go(func() error {
			id, err := common.WaitForLease(ctx, clients, lease, kubeSystem)
			if err != nil {
				return fmt.Errorf("%s has no leader: %w", lease, err)
			}
			s.T().Logf("%s has a leader: %q", lease, id)
			return nil
		})
	}

	for _, daemonSet := range []string{"kube-proxy", "konnectivity-agent"} {
		daemonSet := daemonSet
		eg.Go(func() error {
			if err := common.WaitForDaemonSet(ctx, clients, daemonSet, "kube-system"); err != nil {
				return fmt.Errorf("%s is not ready: %w", daemonSet, err)
			}
			s.T().Log(daemonSet, "is ready")
			return nil
		})
	}

	for _, deployment := range []string{"coredns", "metrics-server"} {
		deployment := deployment
		eg.Go(func() error {
			if err := common.WaitForDeployment(ctx, clients, deployment, "kube-system"); err != nil {
				return fmt.Errorf("%s did not become ready: %w", deployment, err)
			}
			s.T().Log(deployment, "is ready")
			return nil
		})
	}

	return eg.Wait()
}

func TestNodeLocalLoadBalancingSuite(t *testing.T) {
	s := suite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     2,
		},
	}
	testifysuite.Run(t, &s)
}
