// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package nllb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync/atomic"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	testifysuite "github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"
)

type suite struct {
	common.BootlooseSuite
	nllbType   v1beta1.NllbType
	isIPv6Only bool
}

func (s *suite) TestNodeLocalLoadBalancing() {
	const controllerArgs = "--kube-controller-manager-extra-args='--node-monitor-period=3s --node-monitor-grace-period=9s' --feature-gates=IPv6SingleStack=true"

	ctx := s.Context()

	{
		clusterCfg := &v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				Network: func() *v1beta1.Network {
					network := v1beta1.DefaultNetwork()
					network.NodeLocalLoadBalancing.Enabled = true
					if s.nllbType != "" {
						network.NodeLocalLoadBalancing.Type = s.nllbType
					}
					return network
				}(),

				WorkerProfiles: v1beta1.WorkerProfiles{
					v1beta1.WorkerProfile{
						Name: metav1.NamespaceDefault,
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
		}
		if s.isIPv6Only {
			s.T().Log("Running in IPv6-only mode")
			clusterCfg.Spec.Network.PrimaryAddressFamily = v1beta1.PrimaryFamilyIPv6
			clusterCfg.Spec.Network.PodCIDR = "fd00::/108"
			clusterCfg.Spec.Network.ServiceCIDR = "fd01::/108"
		}

		config, err := yaml.Marshal(clusterCfg)
		s.Require().NoError(err)

		for i := range s.ControllerCount {
			s.WriteFileContent(s.ControllerNode(i), "/tmp/k0s.yaml", config)
		}
	}

	s.Run("controller_and_workers_get_up", func() {
		s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", controllerArgs))

		if s.isIPv6Only {
			s.T().Log("Setting up IPv6 DNS for workers")
			common.ConfigureIPv6ResolvConf(&s.BootlooseSuite)
		}

		s.T().Log("Starting workers and waiting for cluster to become ready")

		token, err := s.GetJoinToken("worker")
		s.Require().NoError(err)
		s.Require().NoError(s.RunWorkersWithToken(token))

		restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
		s.Require().NoError(err)
		clients, err := kubernetes.NewForConfig(restConfig)
		s.Require().NoError(err)

		eg, _ := errgroup.WithContext(ctx)
		for i := range s.WorkerCount {
			nodeName := s.WorkerNode(i)
			eg.Go(func() error {
				if err := s.WaitForNodeReady(nodeName, clients); err != nil {
					return fmt.Errorf("Node %s is not ready: %w", nodeName, err)
				}
				return nil
			})
		}
		s.Require().NoError(eg.Wait())

		s.Require().NoError(s.checkClusterReadiness(ctx, restConfig, clients, 1))
	})

	s.Run("join_new_controllers", func() {
		token, err := s.GetJoinToken("controller")
		s.Require().NoError(err)

		eg, _ := errgroup.WithContext(ctx)
		eg.Go(func() error { return s.InitController(1, "--config=/tmp/k0s.yaml", controllerArgs, token) })
		eg.Go(func() error { return s.InitController(2, "--config=/tmp/k0s.yaml", controllerArgs, token) })

		s.Require().NoError(eg.Wait())

		restConfig, err := s.GetKubeConfig(s.ControllerNode(1))
		s.Require().NoError(err)
		clients, err := kubernetes.NewForConfig(restConfig)
		s.Require().NoError(err)

		s.T().Logf("Checking if HA cluster is ready")
		s.Require().NoError(s.checkClusterReadiness(ctx, restConfig, clients, s.ControllerCount))
	})

	// At the time of writing, reaching this point will take ~4m.
	//
	// Each controller iteration below will again take ~4m:
	//
	// - stopping nodes ~30s
	// - Two agent rollouts after controller removal and restart ~100s each
	//
	// So the overall test timeout should be no less than 20m (including 4m
	// buffer).

	workerNameToRestart := s.WorkerNode(0)
	for i := range s.ControllerCount {
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

		restConfig, err := s.GetKubeConfig(s.ControllerNode((i + 1) % s.ControllerCount))
		s.Require().NoError(err)
		clients, err := kubernetes.NewForConfig(restConfig)
		s.Require().NoError(err)

		s.Run("cluster_ready_without_"+controllerName, func() {
			s.Require().NoError(s.checkClusterReadiness(ctx, restConfig, clients, s.ControllerCount, controllerName))
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

			_, err = clients.AppsV1().DaemonSets(metav1.NamespaceSystem).Create(ctx, &dummyDaemons, metav1.CreateOptions{})
			s.Require().NoError(err)

			s.NoError(common.WaitForDaemonSet(s.Context(), clients, name, metav1.NamespaceSystem))
			s.T().Logf("Dummy DaemonSet %s is ready", name)
		})

		s.Run("restart_"+controllerName, func() {
			s.Require().NoError(s.StartController(controllerName))
			clients, err := s.KubeClient(controllerName)
			s.Require().NoError(err)
			s.Require().NoError(s.checkClusterReadiness(ctx, restConfig, clients, s.ControllerCount))
		})
	}

	s.Run("cluster_ready_after_all_controllers_restarted", func() {
		restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
		s.Require().NoError(err)
		clients, err := kubernetes.NewForConfig(restConfig)
		s.Require().NoError(err)
		s.Require().NoError(s.checkClusterReadiness(ctx, restConfig, clients, s.ControllerCount))
	})
}

func (s *suite) checkClusterReadiness(ctx context.Context, restConfig *rest.Config, clients *kubernetes.Clientset, numControllers int, degradedControllers ...string) error {
	eg, ctx := errgroup.WithContext(ctx)

	for i := range numControllers {
		nodeName := s.ControllerNode(i)
		degraded := slices.Contains(degradedControllers, nodeName)

		eg.Go(func() error {
			var holderIdentity string
			watchLeases := watch.FromClient[*coordinationv1.LeaseList, coordinationv1.Lease]

			err := watchLeases(clients.CoordinationV1().Leases(corev1.NamespaceNodeLease)).
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

	var pendingWorkers atomic.Int64
	pendingWorkers.Store(int64(s.WorkerCount))
	workersReady := make(chan struct{})
	for i := range s.WorkerCount {
		nodeName := s.WorkerNode(i)

		eg.Go(func() error {
			if err := common.WaitForNodeReadyStatus(ctx, clients, nodeName, corev1.ConditionTrue); err != nil {
				return fmt.Errorf("node %s did not become ready: %w", nodeName, err)
			}

			s.T().Logf("Node %s is ready", nodeName)

			nllbPodName := "nllb-" + nodeName
			if err := common.WaitForPod(ctx, clients, nllbPodName, metav1.NamespaceSystem); err != nil {
				return fmt.Errorf("Pod %s/%s is not ready: %w", nllbPodName, metav1.NamespaceSystem, err)
			}
			s.T().Logf("Pod %s/%s is ready", metav1.NamespaceSystem, nllbPodName)

			if pendingWorkers.Add(-1) < 1 {
				close(workersReady)
			}

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
		eg.Go(func() error {
			id, err := common.WaitForLease(ctx, clients, lease, metav1.NamespaceSystem)
			if err != nil {
				return fmt.Errorf("%s has no leader: %w", lease, err)
			}
			s.T().Logf("%s has a leader: %q", lease, id)
			return nil
		})
	}

	select {
	case <-workersReady:
	case <-ctx.Done():
		return context.Cause(ctx)
	}

	eg.Go(func() error {
		if err := common.WaitForDaemonSet(ctx, clients, "konnectivity-agent", metav1.NamespaceSystem); err != nil {
			return fmt.Errorf("konnectivity-agent is not ready: %w", err)
		}
		s.T().Log("konnectivity-agent is ready")

		if err := common.VerifyKonnectivityMesh(ctx, restConfig, clients, s.T(), uint(max(0, numControllers-len(degradedControllers))), uint(s.WorkerCount)); err != nil {
			return fmt.Errorf("failed to verify konnectivity mesh: %w", err)
		}
		s.T().Log("Konnectivity mesh is complete")

		return nil
	})

	eg.Go(func() error {
		if err := common.WaitForDaemonSet(ctx, clients, "kube-proxy", metav1.NamespaceSystem); err != nil {
			return fmt.Errorf("kube-proxy is not ready: %w", err)
		}
		s.T().Log("kube-proxy is ready")
		return nil
	})

	for _, deployment := range []string{"coredns", "metrics-server"} {
		eg.Go(func() error {
			if err := common.WaitForDeployment(ctx, clients, deployment, metav1.NamespaceSystem); err != nil {
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
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     2,
		},
	}

	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "traefik") {
		t.Log("Using Traefik")
		s.nllbType = v1beta1.NllbTypeTraefik
	} else {
		t.Log("Using the default NLLB backend")
	}

	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "ipv6") {
		t.Log("Configuring IPv6 only networking")
		s.isIPv6Only = true
		s.Networks = []string{"bridge-ipv6"}
		s.AirgapImageBundleMountPoints = []string{"/var/lib/k0s/images/bundle-ipv6.tar"}
	}

	testifysuite.Run(t, &s)
}
