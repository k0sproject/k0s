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

package quorum

import (
	"context"
	"fmt"
	"testing"
	"time"

	apitcomm "github.com/k0sproject/k0s/inttest/autopilot/common"
	apv1beta2 "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/suite"
)

type quorumSuite struct {
	apitcomm.FootlooseSuite
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
	s.Require().NoError(s.DestroyNetwork(network))
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *quorumSuite) SetupTest() {
	ipAddress := s.GetControllerIPAddress(0)
	var joinToken string

	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))

		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithMultiController, ipAddress))

		// Note that the token is intentionally empty for the first controller
		s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server", joinToken))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))

		// With k0s running, then start autopilot
		s.Require().NoError(s.InitControllerAutopilot(idx, "--kubeconfig=/var/lib/k0s/pki/admin.conf", "--mode=controller"))

		client, err := s.ExtensionsClient(s.ControllerNode(0))
		s.Require().NoError(err)

		_, perr := apcomm.WaitForCRDByName(context.TODO(), client, "plans.autopilot.k0sproject.io", 2*time.Minute)
		s.Require().NoError(perr)
		_, cerr := apcomm.WaitForCRDByName(context.TODO(), client, "controlnodes.autopilot.k0sproject.io", 2*time.Minute)
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
        version: ` + apitcomm.TargetK0sVersion + `
        platforms:
          linux-amd64:
            url: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["k0s"]["url"] + `
            sha256: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["k0s"]["sha256"] + `
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
	s.NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := apcomm.WaitForPlanByName(context.TODO(), client, apconst.AutopilotName, 10*time.Minute, func(obj interface{}) bool {
		if plan, ok := obj.(*apv1beta2.Plan); ok {
			return plan.Status.State == appc.PlanCompleted
		}

		return false
	})

	s.NoError(err)
	s.Equal(appc.PlanCompleted, plan.Status.State)

	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		k0sVersion, err := s.GetK0sVersion(s.ControllerNode(idx))
		s.NoError(err)
		s.Equal("v1.23.3+k0s.1", k0sVersion)
	}
}

// TestQuorumSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestQuorumSuite(t *testing.T) {
	suite.Run(t, &quorumSuite{
		apitcomm.FootlooseSuite{
			ControllerCount:    3,
			WorkerCount:        0,
			ControllerNetworks: []string{network},
			WorkerNetworks:     []string{network},
		},
	})
}
