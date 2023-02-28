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

package airgap

import (
	"fmt"
	"testing"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

type airgapSuite struct {
	common.FootlooseSuite
}

const network = "ap-airgap"

// SetupSuite creates the required network before starting footloose.
func (s *airgapSuite) SetupSuite() {
	s.Require().NoError(s.CreateNetwork(network))
	s.FootlooseSuite.SetupSuite()
}

// TearDownSuite tears down the network created after footloose has finished.
func (s *airgapSuite) TearDownSuite() {
	s.FootlooseSuite.TearDownSuite()
	s.Require().NoError(s.MaybeDestroyNetwork(network))
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *airgapSuite) SetupTest() {
	ctx := s.Context()

	// Note that the token is intentionally empty for the first controller
	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	cClient, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(aptest.WaitForCRDByName(ctx, cClient, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, cClient, "controlnodes"))

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	wClient, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), wClient))
}

func (s *airgapSuite) TestApply() {
	(&common.Airgap{
		SSH:  s.SSH,
		Logf: s.T().Logf,
	}).LockdownMachines(s.Context(),
		s.ControllerNode(0), s.WorkerNode(0),
	)

	planTemplate := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: id123
  timestamp: now
  commands:
    - airgapupdate:
        version: v0.0.0
        platforms:
          linux-amd64:
            url: http://localhost/dist/bundle.tar
          linux-arm64:
            url: http://localhost/dist/bundle.tar
        workers:
          discovery:
            static:
              nodes:
                - worker0
    - k0supdate:
        version: v0.0.0
        forceupdate: true
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s
          linux-arm64:
            url: http://localhost/dist/k0s
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
          workers:
            discovery:
              static:
                nodes:
                  - worker0
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
	_, err = aptest.WaitForPlanState(s.Context(), client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	// We are not confirming the image importing functionality of k0s, but we can get a pretty good idea if it worked.

	// Does the bundle exist on the worker, in the proper directory?
	lsout, err := s.RunCommandWorker(0, "ls /var/lib/k0s/images/bundle.tar")
	s.NoError(err)
	s.NotEmpty(lsout)
}

// TestAirgapSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestAirgapSuite(t *testing.T) {
	suite.Run(t, &airgapSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			LaunchMode:      common.LaunchModeOpenRC,

			AirgapImageBundleMountPoints: []string{"/dist/bundle.tar"},

			ControllerNetworks: []string{network},
			WorkerNetworks:     []string{network},
		},
	})
}
