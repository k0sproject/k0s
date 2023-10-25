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

package clusterreboot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type ClusterRebootSuite struct {
	common.BootlooseSuite
}

func (s *ClusterRebootSuite) TestK0sClusterReboot() {
	s.T().Log("Starting k0s")
	s.NoError(s.InitController(0))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.NoError(s.RunWorkers())
	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	// reboot the cluster:
	s.T().Log("Rebooting cluster")
	s.rebootCluster()

	// Verify things work after the reboot
	kc, err = s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

	// restart k0s worker and make sure it comes up
	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "CNI did not start")
}

// rebootCluster reboots the cluster using bootloose interfaces because
// running reboot on a container won't bring it up automatically:
// https://github.com/weaveworks/footloose/issues/254
func (s *ClusterRebootSuite) rebootCluster() {
	s.Require().NoError(s.Stop([]string{}))
	s.Require().NoError(s.Start([]string{}))

	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 1*time.Minute, 1*time.Second))
	s.Require().NoError(s.WaitForSSH(s.WorkerNode(0), 1*time.Minute, 1*time.Second))
}

func TestClusterRebootSuite(t *testing.T) {
	s := ClusterRebootSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	}
	suite.Run(t, &s)
}
