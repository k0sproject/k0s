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

package updater

import (
	"context"
	"fmt"
	"testing"
	"time"

	apitcomm "github.com/k0sproject/k0s/inttest/autopilot/common"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/suite"
)

const (
	ManifestTestDirPerms = "775"
)

type plansSingleControllerSuite struct {
	apitcomm.FootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *plansSingleControllerSuite) SetupTest() {
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
	_, uerr := apcomm.WaitForCRDByName(context.TODO(), client, "updateconfigs.autopilot.k0sproject.io", 2*time.Minute)
	s.Require().NoError(uerr)
}

// TestApply applies a well-formed `plan` yaml, and asserts that all of the correct values
// across different objects are correct.
func (s *plansSingleControllerSuite) TestApply() {
	updaterConfig := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: UpdateConfig
metadata:
  name: autopilot
spec:
  channel: stable
  updateServer: {{.Address}}
  upgradeStrategy:
    cron: "* * * * * *"
  planSpec:
    id: id123
    timestamp: now
    commands: []
`

	vars := struct {
		Address string
	}{
		Address: fmt.Sprintf("http://%s", s.GetUpdateServerIPAddress()),
	}

	manifestFile := "/tmp/updateconfig.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, updaterConfig, vars)

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

	k0sVersion, err := s.GetK0sVersion(s.ControllerNode(0))
	s.NoError(err)
	s.Equal("v1.23.3+k0s.1", k0sVersion)
}

// TestPlansSingleControllerSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlansSingleControllerSuite(t *testing.T) {
	suite.Run(t, &plansSingleControllerSuite{
		apitcomm.FootlooseSuite{
			ControllerCount:  1,
			WorkerCount:      0,
			WithUpdateServer: true,
		},
	})
}
