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

package multicontroller

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type MultiControllerSuite struct {
	common.BootlooseSuite
}

func (s *MultiControllerSuite) TestK0sGetsUp() {
	ipAddress := s.GetControllerIPAddress(0)
	s.T().Logf("ip address: %s", ipAddress)

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithMultiController, ipAddress))
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	token, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	s.PutFile(s.ControllerNode(1), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithMultiController, ipAddress))
	s.Require().NoError(s.InitController(1, "--config=/tmp/k0s.yaml", token))

	s.PutFile(s.ControllerNode(2), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithMultiController, ipAddress))
	s.Require().NoError(s.InitController(2, "--config=/tmp/k0s.yaml", token))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.Require().NoError(err)

	if !s.AssertSomeKubeSystemPods(kc) {
		return
	}

	s.T().Log("waiting to see CNI pods ready")
	s.Require().NoError(common.WaitForKubeRouterReady(s.Context(), kc), "CNI did not start")
}

func TestMultiControllerSuite(t *testing.T) {
	s := MultiControllerSuite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithMultiController = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  api:
    externalAddress: %s
`
