// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package noderole

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/stretchr/testify/suite"
)

type NodeRoleSuite struct {
	common.BootlooseSuite
}

func (s *NodeRoleSuite) TestK0sGetsUp() {
	ipAddress := s.GetControllerIPAddress(0)
	s.T().Logf("ip address: %s", ipAddress)

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithNodeRole, ipAddress))
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-worker"))

	token, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	s.PutFile(s.ControllerNode(1), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithNodeRole, ipAddress))
	s.NoError(s.InitController(1, "--config=/tmp/k0s.yaml", "--enable-worker", token))

	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeLabel(kc, s.ControllerNode(0), "node-role.kubernetes.io/control-plane", "true")
	s.NoError(err)

	if n, err := kc.CoreV1().Nodes().Get(s.Context(), s.ControllerNode(0), metav1.GetOptions{}); s.NoError(err) {
		s.Contains(n.Spec.Taints, constants.ControlPlaneTaint)
	}

	err = s.WaitForNodeLabel(kc, s.ControllerNode(1), "node-role.kubernetes.io/control-plane", "true")
	s.NoError(err)

	if n, err := kc.CoreV1().Nodes().Get(s.Context(), s.ControllerNode(1), metav1.GetOptions{}); s.NoError(err) {
		s.Contains(n.Spec.Taints, constants.ControlPlaneTaint)
	}

	if n, err := kc.CoreV1().Nodes().Get(s.Context(), s.WorkerNode(0), metav1.GetOptions{}); s.NoError(err) {
		s.NotContains(slices.Collect(maps.Keys(n.Labels)), "node-role.kubernetes.io/master")
		s.False(slices.ContainsFunc(n.Spec.Taints, func(taint corev1.Taint) bool {
			return taint.Key == constants.ControlPlaneTaint.Key
		}), "Worker node has been tainted when it shouldn't")
	}
}

func TestNodeRoleSuite(t *testing.T) {
	s := NodeRoleSuite{
		common.BootlooseSuite{
			ControllerCount: 2,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithNodeRole = `
spec:
  api:
    externalAddress: %s
`
