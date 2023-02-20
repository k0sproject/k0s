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

package kine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type KineSuite struct {
	common.FootlooseSuite
}

func (s *KineSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithKine)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "CNI did not start")

	s.T().Run("verify", func(t *testing.T) {
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		require.NoError(t, err, "failed to SSH into controller")
		defer ssh.Disconnect()

		t.Run(("kineIsUsedAsStorage"), func(t *testing.T) {
			_, err = ssh.ExecWithOutput(s.Context(), "test -e /var/lib/k0s/bin/kine && ps xa | grep kine")
			assert.NoError(t, err)
		})

		t.Run(("noControllerJoinTokens"), func(t *testing.T) {
			noToken, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("'%s' token create --role=controller", s.K0sFullPath))
			assert.Error(t, err)
			assert.Equal(t, "Error: refusing to create token: cannot join controller into current storage", noToken)
		})

		t.Run(("workerJoinTokens"), func(t *testing.T) {
			_, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("'%s' token create --role=worker", s.K0sFullPath))
			assert.NoError(t, err)
		})
	})
}

func TestKineSuite(t *testing.T) {
	s := KineSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithKine = `
spec:
  storage:
    type: kine
`
