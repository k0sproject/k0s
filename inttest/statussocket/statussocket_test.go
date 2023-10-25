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
