// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package statussocket

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type StatusSocketSuite struct {
	common.BootlooseSuite
}

func (s *StatusSocketSuite) TestK0sGetsUp() {
	s.MakeDir(s.ControllerNode(0), "/run/k0s")
	s.PutFile(s.ControllerNode(0), "/run/k0s/status.sock", "")
	s.NoError(s.InitController(0, "--single"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)
}

func TestStatusSocketSuite(t *testing.T) {
	s := StatusSocketSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}
