// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package noderole

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/stretchr/testify/suite"
)

type NodeRoleSingleSuite struct {
	common.BootlooseSuite
}

func (s *NodeRoleSingleSuite) TestK0sSingleNode() {
	err := s.InitController(0, "--single")
	s.Require().NoError(err)

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeLabel(kc, s.ControllerNode(0), "node-role.kubernetes.io/control-plane", "true")
	s.NoError(err)

	if n, err := kc.CoreV1().Nodes().Get(s.Context(), s.ControllerNode(0), metav1.GetOptions{}); s.NoError(err) {
		s.NotContains(n.Spec.Taints, constants.ControlPlaneTaint)
	}
}

func TestNodeRoleSingleSuite(t *testing.T) {
	s := NodeRoleSingleSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}
