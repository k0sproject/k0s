// Copyright 2022 k0s authors
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

package quorumsafety

import (
	"fmt"
	"testing"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
)

type quorumSafetySuite struct {
	common.FootlooseSuite
}

const k0sConfigWithMultiController = `
spec:
  api:
    externalAddress: %s
`

const network = "quorumsafetynet"

// SetupSuite creates the required network before starting footloose.
func (s *quorumSafetySuite) SetupSuite() {
	s.Require().NoError(s.CreateNetwork(network))
	s.FootlooseSuite.SetupSuite()
}

// TearDownSuite tears down the network created after footloose has finished.
func (s *quorumSafetySuite) TearDownSuite() {
	s.FootlooseSuite.TearDownSuite()
	s.Require().NoError(s.MaybeDestroyNetwork(network))
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *quorumSafetySuite) SetupTest() {
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

		_, perr := apcomm.WaitForCRDByName(s.Context(), client, "plans.autopilot.k0sproject.io", 2*time.Minute)
		s.Require().NoError(perr)
		_, cerr := apcomm.WaitForCRDByName(s.Context(), client, "controlnodes.autopilot.k0sproject.io", 2*time.Minute)
		s.Require().NoError(cerr)

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
func (s *quorumSafetySuite) TestApply() {

	// Create a third node by way of a new `ControlNode` entry that doesen't map to a host.
	// This will allow autopilot to get past the node tests in newplan (IncompleteTargets)

	controller2Def := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: ControlNode
metadata:
  name: controller2
  labels:
    kubernetes.io/arch: amd64
    kubernetes.io/hostname: controller2
    kubernetes.io/os: linux
`
	controller2Filename := "/tmp/controller2.yaml"
	s.PutFile(s.ControllerNode(0), controller2Filename, controller2Def)
	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", controller2Filename))
	s.T().Logf("kubectl apply output (controller2): '%s'", out)
	s.Require().NoError(err)

	// Create + populate the plan

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

	out, err = s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output (plan): '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(client)

	// The plan should fail with "InconsistentTargets" due to autopilot detecting that `controller2`
	// despite existing as a `ControlNode`, does not resolve.
	plan, err := apcomm.WaitForPlanByName(s.Context(), client, apconst.AutopilotName, 10*time.Minute, func(plan *apv1beta2.Plan) bool {
		return plan.Status.State == appc.PlanInconsistentTargets
	})

	s.Require().NoError(err)
	s.Equal(appc.PlanInconsistentTargets, plan.Status.State)
}

// TestQuorumSafetySuite sets up a suite using 2 controllers, and runs a specific
// test scenario covering the breaking of quorum.
func TestQuorumSafetySuite(t *testing.T) {
	suite.Run(t, &quorumSafetySuite{
		common.FootlooseSuite{
			ControllerCount: 2,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,

			ControllerNetworks: []string{network},
			WorkerNetworks:     []string{network},
		},
	})
}
