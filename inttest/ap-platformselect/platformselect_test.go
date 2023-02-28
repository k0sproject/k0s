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

package platformselect

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

type platformSelectSuite struct {
	common.FootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *platformSelectSuite) SetupTest() {
	ctx := s.Context()
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))

	s.Require().NoError(s.InitController(0), "--disable-components=metrics-server")
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	client, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "controlnodes"))
}

// TestApply applies a well-formed `plan` yaml that includes multiple
// platform definitions, and asserts that the proper binary is downloaded.
func (s *platformSelectSuite) TestApply() {
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
          windows-amd64:
            url: http://localhost/dist/k0s-windows-amd64
          linux-amd64:
            url: http://localhost/dist/k0s
          linux-arm64:
            url: http://localhost/dist/k0s-arm64
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
`

	manifestFile := "/tmp/happy.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(client)

	// Its expected that if the wrong platform were to be downloaded, the update wouldn't be successful,
	// as the binary would fail to run.

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := aptest.WaitForPlanState(s.Context(), client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	s.Equal(1, len(plan.Status.Commands))
	cmd := plan.Status.Commands[0]

	s.NotNil(cmd.K0sUpdate)
	s.NotNil(cmd.K0sUpdate.Controllers)
	s.NotEmpty(cmd.K0sUpdate.Controllers)
	s.Equal(appc.SignalCompleted, cmd.K0sUpdate.Controllers[0].State)
}

// TestPlatformSelectSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlatformSelectSuite(t *testing.T) {
	suite.Run(t, &platformSelectSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
