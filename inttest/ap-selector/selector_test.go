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

package selector

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/suite"
)

type selectorSuite struct {
	common.FootlooseSuite
}

const selectorControllerConfig = `
spec:
  api:
    externalAddress: %s
`

const network = "selectornet"

// SetupSuite creates the required network before starting footloose.
func (s *selectorSuite) SetupSuite() {
	s.Require().NoError(s.CreateNetwork(network))
	s.FootlooseSuite.SetupSuite()
}

// TearDownSuite tears down the network created after footloose has finished.
func (s *selectorSuite) TearDownSuite() {
	s.FootlooseSuite.TearDownSuite()
	s.Require().NoError(s.DestroyNetwork(network))
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *selectorSuite) SetupTest() {
	ipAddress := s.GetLBAddress()
	var joinToken string

	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))

		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", fmt.Sprintf(selectorControllerConfig, ipAddress))

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

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	for idx := 0; idx < s.FootlooseSuite.WorkerCount; idx++ {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(idx), client))
	}
}

// TestSelectors applies a well-formed `plan` yaml that wants to only update a controller statically, and
// a worker via field/label selector definitions.
func (s *selectorSuite) TestSelectors() {
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
              selector:
                labels: foo=bar
                fields: metadata.name=controller0
          workers:
            discovery:
              selector:
                labels: foo=bar
                fields: metadata.name=worker1
`
	// Add 'foo=bar' to both 'controller0' and 'worker1'
	_, err := s.RunCommandController(0, "/usr/local/bin/k0s kubectl label controlnodes controller0 foo=bar")
	s.Require().NoError(err)
	_, err = s.RunCommandController(0, "/usr/local/bin/k0s kubectl label nodes worker1 foo=bar")
	s.Require().NoError(err)

	// Save + apply the plan
	manifestFile := "/tmp/plan.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	apc, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(apc)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := apcomm.WaitForPlanByName(s.Context(), apc, apconst.AutopilotName, 10*time.Minute, func(plan *apv1beta2.Plan) bool {
		return plan.Status.State == appc.PlanCompleted
	})

	s.Require().NoError(err)
	s.Equal(appc.PlanCompleted, plan.Status.State)

	s.Equal(1, len(plan.Status.Commands))
	cmd := plan.Status.Commands[0]

	s.Equal(appc.PlanCompleted, cmd.State)
	s.NotNil(cmd.K0sUpdate)
	s.NotNil(cmd.K0sUpdate.Controllers)
	s.NotNil(cmd.K0sUpdate.Workers)

	for _, group := range [][]apv1beta2.PlanCommandTargetStatus{cmd.K0sUpdate.Controllers, cmd.K0sUpdate.Workers} {
		for _, node := range group {
			s.Equal(appc.SignalCompleted, node.State)
		}
	}
}

// TestSelectorSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestSelectorSuite(t *testing.T) {
	suite.Run(t, &selectorSuite{
		common.FootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     3,
			WithLB:          true,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
