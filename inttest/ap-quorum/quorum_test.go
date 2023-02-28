// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quorum

import (
	"fmt"
	"testing"
	"time"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

type quorumSuite struct {
	common.FootlooseSuite
}

const k0sConfigWithMultiController = `
spec:
  api:
    externalAddress: %s
`

const network = "quorumnet"

// SetupSuite creates the required network before starting footloose.
func (s *quorumSuite) SetupSuite() {
	s.Require().NoError(s.CreateNetwork(network))
	s.FootlooseSuite.SetupSuite()
}

// TearDownSuite tears down the network created after footloose has finished.
func (s *quorumSuite) TearDownSuite() {
	s.FootlooseSuite.TearDownSuite()
	s.Require().NoError(s.MaybeDestroyNetwork(network))
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *quorumSuite) SetupTest() {
	ctx := s.Context()
	ipAddress := s.GetControllerIPAddress(0)
	var joinToken string

	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))

		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithMultiController, ipAddress))

		// Note that the token is intentionally empty for the first controller
		s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server", joinToken))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))

		client, err := s.ExtensionsClient(s.ControllerNode(0))
		s.Require().NoError(err)

		s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "plans"))
		s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "controlnodes"))

		// With the primary controller running, create the join token for subsequent controllers.
		if idx == 0 {
			token, err := s.GetJoinToken("controller")
			s.Require().NoError(err)
			joinToken = token
		}
	}

	// Final sanity -- ensure all nodes see each other according to etcd
	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		s.Require().Len(s.GetMembers(idx), s.FootlooseSuite.ControllerCount)
	}
}

// TestApply applies a well-formed `plan` yaml, and asserts that
// all of the correct values across different objects + controllers are correct.
func (s *quorumSuite) TestApply() {
	planTemplate := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: id123
  timestamp: now
  commands:
    - k0supdate:
        version: v0.0.0
        forceupdate: true
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
                  - controller1
                  - controller2
`

	manifestFile := "/tmp/happy.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := aptest.WaitForPlanState(s.Context(), client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	s.Equal(1, len(plan.Status.Commands))
	cmd := plan.Status.Commands[0]

	s.Equal(appc.PlanCompleted, cmd.State)
	s.NotNil(cmd.K0sUpdate)
	s.NotNil(cmd.K0sUpdate.Controllers)
	s.Empty(cmd.K0sUpdate.Workers)

	for _, node := range cmd.K0sUpdate.Controllers {
		s.Equal(appc.SignalCompleted, node.State)
	}
}

// TestQuorumSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestQuorumSuite(t *testing.T) {
	suite.Run(t, &quorumSuite{
		common.FootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,

			ControllerNetworks: []string{network},
			WorkerNetworks:     []string{network},
		},
	})
}
