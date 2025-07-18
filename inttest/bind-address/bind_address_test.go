// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package bind_address

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func (s *suite) TestCustomizedBindAddress() {
	const controllerArgs = "--kube-controller-manager-extra-args='--node-monitor-period=3s --node-monitor-grace-period=9s'"

	ctx := s.Context()

	{
		for i := range s.ControllerCount {
			config, err := yaml.Marshal(&v1beta1.ClusterConfig{
				Spec: &v1beta1.ClusterSpec{
					API: func() *v1beta1.APISpec {
						apiSpec := v1beta1.DefaultAPISpec()
						apiSpec.Address = s.GetIPAddress(s.ControllerNode(i))
						apiSpec.OnlyBindToAddress = true
						return apiSpec
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

		s.Require().NoError(s.checkClusterReadiness(ctx, clients))
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
		s.Require().NoError(s.checkClusterReadiness(ctx, clients))
	})
}

func (s *suite) checkClusterReadiness(ctx context.Context, clients *kubernetes.Clientset) error {
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := common.WaitForKubeRouterReady(ctx, clients); err != nil {
			return fmt.Errorf("kube-router did not start: %w", err)
		}
		s.T().Logf("kube-router is ready")
		return nil
	})

	for _, lease := range []string{"kube-scheduler", "kube-controller-manager"} {
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
		eg.Go(func() error {
			if err := common.WaitForDaemonSet(ctx, clients, daemonSet, "kube-system"); err != nil {
				return fmt.Errorf("%s is not ready: %w", daemonSet, err)
			}
			s.T().Log(daemonSet, "is ready")
			return nil
		})
	}

	for _, deployment := range []string{"coredns", "metrics-server"} {
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

func TestCustomizedBindAddressSuite(t *testing.T) {
	s := suite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     1,
		},
	}
	testifysuite.Run(t, &s)
}
