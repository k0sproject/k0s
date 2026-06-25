// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package basic

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CapitalHostnamesSuite struct {
	common.BootlooseSuite
}

func (s *CapitalHostnamesSuite) TestK0sGetsUp() {
	ctx := s.Context()

	s.NoError(s.setHostname(s.ControllerNode(0), "k0s-CONTROLLER"))
	s.NoError(s.setHostname(s.WorkerNode(0), "k0s-WORKER"))

	s.NoError(s.InitController(0))

	token, err := s.GetJoinToken("worker")
	s.Require().NoError(err)
	s.NoError(s.RunWorkersWithToken(token))

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	kc, err := kubernetes.NewForConfig(restConfig)
	s.Require().NoError(err)

	err = s.WaitForNodeReady("k0s-worker", kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(ctx, kc), "kube-router did not start")

	s.T().Log("waiting for konnectivity")
	s.Require().NoError(common.VerifyKonnectivityMesh(ctx, restConfig, kc, s.T(), uint(s.ControllerCount), uint(s.WorkerCount)), "While verifying konnectivity mesh")

	// Verify API that we get proper controller counter lease
	_, err = kc.CoordinationV1().Leases(corev1.NamespaceNodeLease).Get(ctx, "k0s-ctrl-k0s-controller", metav1.GetOptions{})
	s.NoError(err)

	// Verify the autopilot controller node is created
	apClient, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(apClient)
	_, err = apClient.AutopilotV1beta2().ControlNodes().Get(ctx, "k0s-controller", metav1.GetOptions{})
	s.NoError(err)
}

func (s *CapitalHostnamesSuite) setHostname(node, hostname string) error {
	ssh, err := s.SSH(s.Context(), node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(s.Context(), "hostname "+hostname)
	return err
}

func TestCapitalHostnamesSuite(t *testing.T) {
	s := CapitalHostnamesSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
