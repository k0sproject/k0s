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

package ha3x3

import (
	"fmt"
	"strings"
	"testing"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

type ha3x3Suite struct {
	common.FootlooseSuite
}

const haControllerConfig = `
spec:
  api:
    externalAddress: %s
`

const network = "ha3x3net"

// SetupSuite creates the required network before starting footloose.
func (s *ha3x3Suite) SetupSuite() {
	s.Require().NoError(s.CreateNetwork(network))
	s.FootlooseSuite.SetupSuite()
}

// TearDownSuite tears down the network created after footloose has finished.
func (s *ha3x3Suite) TearDownSuite() {
	s.FootlooseSuite.TearDownSuite()
	s.Require().NoError(s.MaybeDestroyNetwork(network))
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *ha3x3Suite) SetupTest() {
	ctx := s.Context()
	ipAddress := s.GetLBAddress()
	var joinToken string

	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))
		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", fmt.Sprintf(haControllerConfig, ipAddress))

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

// TestApply applies a well-formed `plan` yaml, and asserts that
// all of the correct values across different objects + controllers are correct.
func (s *ha3x3Suite) TestApply() {
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
        version: ` + s.K0sUpdateVersion + `
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s-new
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
                  - controller1
                  - controller2
          workers:
            discovery:
              static:
                nodes:
                  - worker0
                  - worker1
                  - worker2
`

	manifestFile := "/tmp/happy.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	ssh, err := s.SSH(s.Context(), s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	out, err = ssh.ExecWithOutput(s.Context(), "/var/lib/k0s/bin/iptables-save -V")
	s.Require().NoError(err)
	iptablesVersionParts := strings.Split(out, " ")
	iptablesModeBeforeUpdate := iptablesVersionParts[len(iptablesVersionParts)-1]

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := aptest.WaitForPlanState(s.Context(), client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	// Ensure all state/status are completed
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

	if version, err := s.GetK0sVersion(s.ControllerNode(0)); s.NoError(err) {
		s.Equal(s.K0sUpdateVersion, version)
	}

	out, err = ssh.ExecWithOutput(s.Context(), "/var/lib/k0s/bin/iptables-save -V")
	s.Require().NoError(err)
	iptablesVersionParts = strings.Split(out, " ")
	iptablesModeAfterUpdate := iptablesVersionParts[len(iptablesVersionParts)-1]
	s.Equal(iptablesModeBeforeUpdate, iptablesModeAfterUpdate)
}

// TestHA3x3Suite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestHA3x3Suite(t *testing.T) {
	suite.Run(t, &ha3x3Suite{
		common.FootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     3,
			WithLB:          true,
			LaunchMode:      common.LaunchModeOpenRC,

			ControllerNetworks: []string{network},
			WorkerNetworks:     []string{network},
		},
	})
}
