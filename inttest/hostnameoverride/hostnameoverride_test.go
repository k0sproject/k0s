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

package hostnameoverride

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
)

type hostnameOverrideSuite struct {
	common.BootlooseSuite
}

func (s *hostnameOverrideSuite) TestK0sGetsUp() {
	s.Require().NoError(s.InitController(0, "--disable-components=konnectivity-server,metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// Create a worker join token
	joinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(joinToken, "--kubelet-extra-args=--hostname-override=foobar"))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady("foobar", client))
}

func TestHostnameOverrideSuite(t *testing.T) {
	suite.Run(t, &hostnameOverrideSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	})
}
