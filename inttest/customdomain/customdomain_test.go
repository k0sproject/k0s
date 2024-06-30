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

package customdomain

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type CustomDomainSuite struct {
	common.BootlooseSuite
}

func (s *CustomDomainSuite) TestK0sGetsUpWithCustomDomain() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	// Metrics disabled as it's super slow to get up properly and interferes with API discovery etc. while it's getting up
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components metrics-server"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "CNI did not start")

	s.Run("check custom domain existence in pod", func() {
		// All done via SSH as it's much simpler :)
		// e.g. execing via client-go is super complex and would require too much wiring
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		s.Require().NoError(err)
		defer ssh.Disconnect()
		_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kc run nginx --image docker.io/nginx:1-alpine")
		s.Require().NoError(err)
		s.NoError(common.WaitForPod(s.Context(), kc, "nginx", "default"))
		s.NoError(common.WaitForPodLogs(s.Context(), kc, "default"))
		output, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kc exec nginx -- cat /etc/resolv.conf")
		s.Require().NoError(err)
		s.Contains(output, "search default.svc.something.local svc.something.local something.local")
	})
}

func TestCustomDomainSuite(t *testing.T) {
	s := CustomDomainSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfig = `
spec:
  storage:
    type: kine
  network:
    clusterDomain: something.local
`
