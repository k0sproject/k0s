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

package platformselect

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

type platformSelectSuite struct {
	apitcomm.FootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *platformSelectSuite) SetupTest() {
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))

	s.Require().NoError(s.InitController(0), "--disable-components=metrics-server")
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// With k0s running, then launch autopilot
	s.Require().NoError(s.InitControllerAutopilot(0, "--kubeconfig=/var/lib/k0s/pki/admin.conf", "--mode=controller"))

	client, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	_, perr := apcomm.WaitForCRDByName(context.TODO(), client, "plans.autopilot.k0sproject.io", 2*time.Minute)
	s.Require().NoError(perr)
	_, cerr := apcomm.WaitForCRDByName(context.TODO(), client, "controlnodes.autopilot.k0sproject.io", 2*time.Minute)
	s.Require().NoError(cerr)
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
        version: ` + apitcomm.TargetK0sVersion + `
        platforms:
          windows-amd64:
            url: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["windows-amd64"]["k0s"]["url"] + `
            sha256: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["windows-amd64"]["k0s"]["sha256"] + `
          linux-amd64:
            url: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["k0s"]["url"] + `
            sha256: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["k0s"]["sha256"] + `
          linux-arm64:
            url: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-arm64"]["k0s"]["url"] + `
            sha256: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-arm64"]["k0s"]["sha256"] + `
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
	s.NoError(err)
	s.NotEmpty(client)

	// Its expected that if the wrong platform were to be downloaded, the update wouldn't be successful,
	// as the binary would fail to run.

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := apcomm.WaitForPlanByName(context.TODO(), client, apconst.AutopilotName, 10*time.Minute, func(obj interface{}) bool {
		if plan, ok := obj.(*apv1beta2.Plan); ok {
			return plan.Status.State == appc.PlanCompleted
		}

		return false
	})

	s.NoError(err)
	s.Equal(appc.PlanCompleted, plan.Status.State)

	k0sVersion, err := s.GetK0sVersion(s.ControllerNode(0))
	s.NoError(err)
	s.Equal("v1.23.3+k0s.1", k0sVersion)
}

// TestPlatformSelectSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlatformSelectSuite(t *testing.T) {
	suite.Run(t, &platformSelectSuite{
		apitcomm.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
		},
	})
}
