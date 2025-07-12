// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
