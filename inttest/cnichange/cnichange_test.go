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

package cnichange

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type CNIChangeSuite struct {
	common.FootlooseSuite
}

func (s *CNIChangeSuite) TestK0sGetsUpButRejectsToChangeCNI() {
	// Run controller with defaults only --> kube-router in use
	s.NoError(s.InitController(0))

	// Restart the controller with new config, should fail as the CNI change is not supported
	sshC1, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err)
	defer sshC1.Disconnect()
	s.T().Log("killing k0s")
	_, err = sshC1.ExecWithOutput(s.Context(), "kill $(pidof k0s) && while pidof k0s; do sleep 0.1s; done")
	s.Require().NoError(err)

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.T().Log("restarting k0s with new cni, this should fail")
	_, err = sshC1.ExecWithOutput(s.Context(), "/usr/local/bin/k0s controller --debug --config /tmp/k0s.yaml")
	s.Require().Error(err)
}

func TestCNIChangeSuite(t *testing.T) {
	s := CNIChangeSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
		},
	}
	suite.Run(t, &s)
}

const k0sConfig = `
spec:
  network:
    provider: calico
    calico:
`
