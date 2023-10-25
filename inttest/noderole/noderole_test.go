/*
Copyright 2021 k0s authors

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

package noderole

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
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

	if n, err := kc.CoreV1().Nodes().Get(s.Context(), s.ControllerNode(0), v1.GetOptions{}); s.NoError(err) {
		s.Contains(n.Spec.Taints, corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: "NoSchedule"})
	}

	err = s.WaitForNodeLabel(kc, s.ControllerNode(1), "node-role.kubernetes.io/control-plane", "true")
	s.NoError(err)

	if n, err := kc.CoreV1().Nodes().Get(s.Context(), s.ControllerNode(1), v1.GetOptions{}); s.NoError(err) {
		s.Contains(n.Spec.Taints, corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: "NoSchedule"})
	}

	if n, err := kc.CoreV1().Nodes().Get(s.Context(), s.WorkerNode(0), v1.GetOptions{}); s.NoError(err) {
		s.NotContains(n.Labels, map[string]string{"node-role.kubernetes.io/master": "NoSchedule"})
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
