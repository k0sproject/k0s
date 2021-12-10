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
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
)

type NodeRoleSingleNoTaintsSuite struct {
	common.FootlooseSuite
}

func (s *NodeRoleSingleNoTaintsSuite) TestK0sSingleNodeNoTaints() {
	s.InitController(0, "--single", "--no-taints")

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	n, err := kc.CoreV1().Nodes().Get(context.TODO(), s.ControllerNode(0), v1.GetOptions{})
	s.NoError(err)
	s.NotContains(n.Spec.Taints, corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: "NoSchedule"})

	err = s.WaitForNodeLabel(kc, s.ControllerNode(0), "node-role.kubernetes.io/control-plane", "true")
	s.NoError(err)
}

func TestNodeRoleSingleNoTaintsSuite(t *testing.T) {
	s := NodeRoleSingleNoTaintsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}
