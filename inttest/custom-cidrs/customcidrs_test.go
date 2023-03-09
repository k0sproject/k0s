/*
Copyright 2020 k0s authors

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

package customcidrs

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
)

type CustomCIDRsSuite struct {
	common.FootlooseSuite
}

const k0sConfig = `
spec:
  storage:
    type: kine
  network:
    serviceCIDR: 10.152.184.0/24
    podCIDR: 10.3.0.0/16
`

func (s *CustomCIDRsSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	// Metrics disabled as it's super slow to get up properly and interferes with API discovery etc. while it's getting up
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components metrics-server", "--enable-dynamic-config"))
	s.Require().NoError(s.RunWorkers())

	token, err := s.GetJoinToken("worker")
	s.Require().NoError(err)
	s.NoError(s.RunWorkersWithToken(token))

	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		s.FailNow("failed to obtain Kubernetes client", err)
	}

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

}

func TestCustomCIDRsSuite(t *testing.T) {
	s := CustomCIDRsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
